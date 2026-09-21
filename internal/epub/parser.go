package epub

import (
	"archive/zip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
)

type Reader struct {
	zip      *zip.ReadCloser
	opfDir   string
	manifest map[string]ManifestItem
	spine    []SpineItem
	meta     BookInfo
	toc      []TOCEntry
}

type containerXML struct {
	XMLName   xml.Name `xml:"container"`
	Rootfiles struct {
		Rootfile []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfile"`
	} `xml:"rootfiles"`
}

type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
}

type opfMetadata struct {
	Title    []string `xml:"title"`
	Creator  []string `xml:"creator"`
	Language []string `xml:"language"`
	Meta     []struct {
		Name    string `xml:"name,attr"`
		Content string `xml:"content,attr"`
	} `xml:"meta"`
}

type opfManifest struct {
	Items []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"item"`
}

type opfSpine struct {
	Toc      string `xml:"toc,attr"`
	Itemrefs []struct {
		IDRef string `xml:"idref,attr"`
	} `xml:"itemref"`
}

type ncxDoc struct {
	XMLName xml.Name `xml:"ncx"`
	NavMap  struct {
		NavPoints []ncxNavPoint `xml:"navPoint"`
	} `xml:"navMap"`
}

type ncxNavPoint struct {
	Label struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Children []ncxNavPoint `xml:"navPoint"`
}

type navDoc struct {
	XMLName xml.Name `xml:"html"`
	Body    struct {
		Navs []navElement `xml:"nav"`
	} `xml:"body"`
}

type navElement struct {
	Type string  `xml:"type,attr"`
	OL   navList `xml:"ol"`
}

type navList struct {
	Items []navItem `xml:"li"`
}

type navItem struct {
	Link struct {
		Href string `xml:"href,attr"`
		Text string `xml:",chardata"`
	} `xml:"a"`
	SubList *navList `xml:"ol"`
}

func Open(filePath string) (*Reader, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open epub zip: %w", err)
	}

	r := &Reader{
		zip:      zr,
		manifest: make(map[string]ManifestItem),
	}

	opfPath, err := r.parseContainer()
	if err != nil {
		zr.Close()
		return nil, fmt.Errorf("parse container.xml: %w", err)
	}

	r.opfDir = path.Dir(opfPath)
	if r.opfDir == "." {
		r.opfDir = ""
	}

	if err := r.parseOPF(opfPath); err != nil {
		zr.Close()
		return nil, fmt.Errorf("parse OPF: %w", err)
	}

	r.parseTOC()

	return r, nil
}

func (r *Reader) Close() error {
	if r.zip != nil {
		return r.zip.Close()
	}
	return nil
}

func (r *Reader) Info() BookInfo {
	return r.meta
}

func (r *Reader) TOC() []TOCEntry {
	return r.toc
}

func (r *Reader) SpineItems() []SpineItem {
	return r.spine
}

func (r *Reader) ChapterHTML(spineIndex int) (string, error) {
	if spineIndex < 0 || spineIndex >= len(r.spine) {
		return "", fmt.Errorf("spine index %d out of range [0, %d)", spineIndex, len(r.spine))
	}

	href := r.spine[spineIndex].Href
	fullPath := r.resolvePath(href)

	data, err := r.readFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read chapter %q: %w", fullPath, err)
	}

	html := string(data)
	html = r.rewriteResources(html, path.Dir(fullPath))

	return html, nil
}

func (r *Reader) CoverImage() string {
	coverID := r.findCoverID()

	if coverID != "" {
		if item, ok := r.manifest[coverID]; ok {
			return r.fileToDataURI(r.resolvePath(item.Href))
		}
	}

	for _, item := range r.manifest {
		if strings.Contains(item.Properties, "cover-image") {
			return r.fileToDataURI(r.resolvePath(item.Href))
		}
	}

	for _, item := range r.manifest {
		if strings.HasPrefix(item.MediaType, "image/") {
			lower := strings.ToLower(item.Href)
			if strings.Contains(lower, "cover") {
				return r.fileToDataURI(r.resolvePath(item.Href))
			}
		}
	}

	for _, item := range r.manifest {
		if strings.HasPrefix(item.MediaType, "image/") {
			return r.fileToDataURI(r.resolvePath(item.Href))
		}
	}

	return ""
}

func (r *Reader) parseContainer() (string, error) {
	data, err := r.readFile("META-INF/container.xml")
	if err != nil {
		return "", err
	}

	var c containerXML
	if err := xml.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("unmarshal container.xml: %w", err)
	}

	if len(c.Rootfiles.Rootfile) == 0 {
		return "", fmt.Errorf("no rootfile found in container.xml")
	}

	return c.Rootfiles.Rootfile[0].FullPath, nil
}

func (r *Reader) parseOPF(opfPath string) error {
	data, err := r.readFile(opfPath)
	if err != nil {
		return err
	}

	var pkg opfPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("unmarshal OPF: %w", err)
	}

	if len(pkg.Metadata.Title) > 0 {
		r.meta.Title = pkg.Metadata.Title[0]
	}
	if len(pkg.Metadata.Creator) > 0 {
		r.meta.Author = pkg.Metadata.Creator[0]
	}
	if len(pkg.Metadata.Language) > 0 {
		r.meta.Language = pkg.Metadata.Language[0]
	}

	for _, item := range pkg.Manifest.Items {
		r.manifest[item.ID] = ManifestItem{
			ID:         item.ID,
			Href:       item.Href,
			MediaType:  item.MediaType,
			Properties: item.Properties,
		}
	}

	for _, ref := range pkg.Spine.Itemrefs {
		if item, ok := r.manifest[ref.IDRef]; ok {
			r.spine = append(r.spine, SpineItem{
				IDRef: ref.IDRef,
				Href:  item.Href,
			})
		}
	}
	r.meta.SpineCount = len(r.spine)
	r.meta.CoverBase64 = r.CoverImage()

	return nil
}

func (r *Reader) findCoverID() string {
	opfPath := ""
	containerData, err := r.readFile("META-INF/container.xml")
	if err != nil {
		return ""
	}
	var c containerXML
	if err := xml.Unmarshal(containerData, &c); err != nil || len(c.Rootfiles.Rootfile) == 0 {
		return ""
	}
	opfPath = c.Rootfiles.Rootfile[0].FullPath

	data, err := r.readFile(opfPath)
	if err != nil {
		return ""
	}

	var pkg opfPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	for _, m := range pkg.Metadata.Meta {
		if m.Name == "cover" && m.Content != "" {
			return m.Content
		}
	}
	return ""
}

func (r *Reader) parseTOC() {
	for _, item := range r.manifest {
		if strings.Contains(item.Properties, "nav") {
			toc, err := r.parseNav(r.resolvePath(item.Href))
			if err == nil && len(toc) > 0 {
				r.toc = toc
				r.resolveSpineIndices()
				return
			}
		}
	}

	for _, item := range r.manifest {
		if item.MediaType == "application/x-dtbncx+xml" {
			toc, err := r.parseNCX(r.resolvePath(item.Href))
			if err == nil && len(toc) > 0 {
				r.toc = toc
				r.resolveSpineIndices()
				return
			}
		}
	}
}

func (r *Reader) parseNav(filePath string) ([]TOCEntry, error) {
	data, err := r.readFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	content = strings.ReplaceAll(content, "epub:", "")

	var doc navDoc
	if err := xml.Unmarshal([]byte(content), &doc); err != nil {
		return r.parseNavFallback(data, filePath)
	}

	for _, nav := range doc.Body.Navs {
		if nav.Type == "toc" {
			return r.convertNavList(nav.OL, path.Dir(filePath)), nil
		}
	}

	if len(doc.Body.Navs) > 0 {
		return r.convertNavList(doc.Body.Navs[0].OL, path.Dir(filePath)), nil
	}

	return nil, fmt.Errorf("no toc nav found")
}

func (r *Reader) parseNavFallback(data []byte, filePath string) ([]TOCEntry, error) {
	content := string(data)
	var entries []TOCEntry

	parts := strings.Split(content, "<a ")
	for _, part := range parts[1:] {
		href := extractAttr(part, "href")
		text := extractTextContent(part)
		if href != "" && text != "" {
			entries = append(entries, TOCEntry{
				Title: strings.TrimSpace(text),
				Href:  href,
			})
		}
	}

	return entries, nil
}

func (r *Reader) convertNavList(ol navList, baseDir string) []TOCEntry {
	var entries []TOCEntry
	for _, item := range ol.Items {
		entry := TOCEntry{
			Title: strings.TrimSpace(item.Link.Text),
			Href:  item.Link.Href,
		}
		if item.SubList != nil {
			entry.Children = r.convertNavList(*item.SubList, baseDir)
		}
		if entry.Title != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (r *Reader) parseNCX(filePath string) ([]TOCEntry, error) {
	data, err := r.readFile(filePath)
	if err != nil {
		return nil, err
	}

	var doc ncxDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal NCX: %w", err)
	}

	return r.convertNCXPoints(doc.NavMap.NavPoints), nil
}

func (r *Reader) convertNCXPoints(points []ncxNavPoint) []TOCEntry {
	var entries []TOCEntry
	for _, p := range points {
		entry := TOCEntry{
			Title: strings.TrimSpace(p.Label.Text),
			Href:  p.Content.Src,
		}
		if len(p.Children) > 0 {
			entry.Children = r.convertNCXPoints(p.Children)
		}
		if entry.Title != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (r *Reader) resolveSpineIndices() {
	hrefToIndex := make(map[string]int)
	for i, s := range r.spine {
		href := s.Href
		if idx := strings.Index(href, "#"); idx != -1 {
			href = href[:idx]
		}
		hrefToIndex[href] = i
	}

	r.resolveIndicesRecursive(r.toc, hrefToIndex)
}

func (r *Reader) resolveIndicesRecursive(entries []TOCEntry, hrefToIndex map[string]int) {
	for i := range entries {
		href := entries[i].Href
		if idx := strings.Index(href, "#"); idx != -1 {
			href = href[:idx]
		}
		if idx, ok := hrefToIndex[href]; ok {
			entries[i].SpineIndex = idx
		}
		if len(entries[i].Children) > 0 {
			r.resolveIndicesRecursive(entries[i].Children, hrefToIndex)
		}
	}
}

func (r *Reader) resolvePath(href string) string {
	if r.opfDir == "" {
		return href
	}
	return r.opfDir + "/" + href
}

func (r *Reader) readFile(name string) ([]byte, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Clean(name)

	for _, f := range r.zip.File {
		fName := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.EqualFold(fName, name) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file %q not found in epub", name)
}

func (r *Reader) fileToDataURI(filePath string) string {
	data, err := r.readFile(filePath)
	if err != nil {
		return ""
	}
	mimeType := http.DetectContentType(data)
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".svg":
		mimeType = "image/svg+xml"
	case ".webp":
		mimeType = "image/webp"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func (r *Reader) rewriteResources(html string, chapterDir string) string {
	html = r.rewriteAttr(html, "src", chapterDir)
	html = r.rewriteAttr(html, "href", chapterDir)
	return html
}

func (r *Reader) rewriteAttr(html string, attr string, chapterDir string) string {
	search := attr + `="`
	var result strings.Builder
	result.Grow(len(html))

	for {
		idx := strings.Index(html, search)
		if idx == -1 {
			result.WriteString(html)
			break
		}

		result.WriteString(html[:idx+len(search)])
		html = html[idx+len(search):]

		endIdx := strings.Index(html, `"`)
		if endIdx == -1 {
			result.WriteString(html)
			break
		}

		value := html[:endIdx]
		html = html[endIdx:]

		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") ||
			strings.HasPrefix(value, "data:") || strings.HasPrefix(value, "#") ||
			strings.HasPrefix(value, "mailto:") {
			result.WriteString(value)
			continue
		}

		resolved := path.Clean(chapterDir + "/" + value)
		dataURI := r.fileToDataURI(resolved)
		if dataURI != "" {
			result.WriteString(dataURI)
		} else {
			result.WriteString(value)
		}
	}

	return result.String()
}

func extractAttr(s string, attr string) string {
	search := attr + `="`
	idx := strings.Index(s, search)
	if idx == -1 {
		search = attr + `='`
		idx = strings.Index(s, search)
		if idx == -1 {
			return ""
		}
	}
	s = s[idx+len(search):]
	end := strings.IndexByte(s, s[0]-1)
	if search[len(search)-1] == '"' {
		end = strings.IndexByte(s, '"')
	} else {
		end = strings.IndexByte(s, '\'')
	}
	if end == -1 {
		return ""
	}
	return s[:end]
}

func extractTextContent(s string) string {
	start := strings.IndexByte(s, '>')
	if start == -1 {
		return ""
	}
	s = s[start+1:]
	end := strings.Index(s, "</")
	if end == -1 {
		end = strings.Index(s, "<")
		if end == -1 {
			return s
		}
	}
	return strings.TrimSpace(s[:end])
}

func (r *Reader) ChapterWordCount(spineIndex int) int {
	if spineIndex < 0 || spineIndex >= len(r.spine) {
		return 0
	}
	fullPath := r.resolvePath(r.spine[spineIndex].Href)
	data, err := r.readFile(fullPath)
	if err != nil {
		return 0
	}
	plainText := stripHTML(string(data))
	return len(strings.Fields(plainText))
}

func (r *Reader) Search(query string) []SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}
	}

	lowerQuery := strings.ToLower(query)
	var results []SearchResult

	for idx, item := range r.spine {
		fullPath := r.resolvePath(item.Href)
		data, err := r.readFile(fullPath)
		if err != nil {
			continue
		}

		plainText := stripHTML(string(data))
		lowerText := strings.ToLower(plainText)

		pos := 0
		matchCount := 0
		var firstSnippet string

		for {
			matchIdx := strings.Index(lowerText[pos:], lowerQuery)
			if matchIdx == -1 {
				break
			}
			absIdx := pos + matchIdx
			matchCount++

			if firstSnippet == "" {
				start := absIdx - 40
				if start < 0 {
					start = 0
				}
				end := absIdx + len(query) + 40
				if end > len(plainText) {
					end = len(plainText)
				}
				snippet := plainText[start:end]
				if start > 0 {
					snippet = "…" + snippet
				}
				if end < len(plainText) {
					snippet = snippet + "…"
				}
				firstSnippet = snippet
			}

			pos = absIdx + len(lowerQuery)
			if pos >= len(lowerText) {
				break
			}
		}

		if matchCount > 0 {
			chTitle := r.getChapterTitle(idx)
			results = append(results, SearchResult{
				SpineIndex:   idx,
				ChapterTitle: chTitle,
				Snippet:      firstSnippet,
				MatchCount:   matchCount,
			})
		}
	}

	if results == nil {
		results = []SearchResult{}
	}
	return results
}

func (r *Reader) getChapterTitle(spineIndex int) string {
	var findInTOC func(entries []TOCEntry) string
	findInTOC = func(entries []TOCEntry) string {
		for _, e := range entries {
			if e.SpineIndex == spineIndex && e.Title != "" {
				return e.Title
			}
			if len(e.Children) > 0 {
				if t := findInTOC(e.Children); t != "" {
					return t
				}
			}
		}
		return ""
	}

	if t := findInTOC(r.toc); t != "" {
		return t
	}
	return fmt.Sprintf("Chapter %d", spineIndex+1)
}

func stripHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			b.WriteByte(' ')
			continue
		}
		if !inTag {
			b.WriteByte(c)
		}
	}
	res := b.String()
	res = strings.ReplaceAll(res, "&nbsp;", " ")
	res = strings.ReplaceAll(res, "&#160;", " ")
	res = strings.ReplaceAll(res, "&amp;", "&")
	res = strings.ReplaceAll(res, "&lt;", "<")
	res = strings.ReplaceAll(res, "&gt;", ">")
	res = strings.ReplaceAll(res, "&quot;", "\"")
	res = strings.ReplaceAll(res, "&apos;", "'")
	return strings.Join(strings.Fields(res), " ")
}
