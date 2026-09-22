#!/usr/bin/env python3
import os
import shutil
import stat
import subprocess
import urllib.request
import cairo
import gi

gi.require_version('Rsvg', '2.0')
from gi.repository import Rsvg

ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
BUILD_BIN = os.path.join(ROOT_DIR, "build", "bin", "read-e")
SVG_ICON = os.path.join(ROOT_DIR, "build", "appicon.svg")
DIST_DIR = os.path.join(ROOT_DIR, "dist")
APPDIR = os.path.join(DIST_DIR, "AppDir")
TOOL_PATH = os.path.join(DIST_DIR, "appimagetool-x86_64.AppImage")

VERSION = "1.0.0"
ARCH = "x86_64"
APPIMAGE_NAME = f"read-e_{VERSION}_{ARCH}.AppImage"
APPIMAGE_OUT = os.path.join(DIST_DIR, APPIMAGE_NAME)

print(f"==> Packaging read e v{VERSION} as portable AppImage ({ARCH})...")

# Check binary exists
if not os.path.isfile(BUILD_BIN):
    print(f"Error: Binary not found at {BUILD_BIN}. Run 'wails build' first.")
    exit(1)

# Clean previous build
if os.path.exists(APPDIR):
    shutil.rmtree(APPDIR)
os.makedirs(DIST_DIR, exist_ok=True)

# Create AppDir structure
bin_dir = os.path.join(APPDIR, "usr", "bin")
apps_dir = os.path.join(APPDIR, "usr", "share", "applications")
hicolor_dir = os.path.join(APPDIR, "usr", "share", "icons", "hicolor")

os.makedirs(bin_dir, exist_ok=True)
os.makedirs(apps_dir, exist_ok=True)

# 1. Copy Binary
dest_bin = os.path.join(bin_dir, "read-e")
shutil.copy2(BUILD_BIN, dest_bin)
os.chmod(dest_bin, 0o755)
print(f"  [+] Installed binary to AppDir/usr/bin/read-e")

# 2. Scalable SVG Icon
scalable_dir = os.path.join(hicolor_dir, "scalable", "apps")
os.makedirs(scalable_dir, exist_ok=True)
shutil.copy2(SVG_ICON, os.path.join(scalable_dir, "read-e.svg"))
shutil.copy2(SVG_ICON, os.path.join(APPDIR, "read-e.svg"))
print(f"  [+] Installed SVG icon")

# 3. Render PNG icons (16, 32, 48, 64, 128, 256, 512)
handle = Rsvg.Handle.new_from_file(SVG_ICON)
dim = handle.get_dimensions()
orig_w, orig_h = dim.width, dim.height

for size in [16, 32, 48, 64, 128, 256, 512]:
    icon_dir = os.path.join(hicolor_dir, f"{size}x{size}", "apps")
    os.makedirs(icon_dir, exist_ok=True)
    out_png = os.path.join(icon_dir, "read-e.png")

    surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, size, size)
    ctx = cairo.Context(surface)
    ctx.scale(size / orig_w, size / orig_h)
    handle.render_cairo(ctx)
    surface.write_to_png(out_png)

    # 256x256 as root icon
    if size == 256:
        shutil.copy2(out_png, os.path.join(APPDIR, "read-e.png"))
        shutil.copy2(out_png, os.path.join(APPDIR, ".DirIcon"))

print(f"  [+] Rendered PNG icons and root icon")

# 4. Desktop Entry
desktop_content = f"""[Desktop Entry]
Name=read e
GenericName=E-Book Reader
Comment=Modern EPUB & PDF Reader with Reading Insights
Exec=read-e %U
Icon=read-e
Terminal=false
Type=Application
Categories=Office;Viewer;
MimeType=application/epub+zip;application/pdf;
Keywords=read;epub;pdf;ebook;reader;books;insights;
StartupWMClass=read-e
"""
desktop_file = os.path.join(apps_dir, "read-e.desktop")
with open(desktop_file, "w", encoding="utf-8") as f:
    f.write(desktop_content)
os.chmod(desktop_file, 0o644)
shutil.copy2(desktop_file, os.path.join(APPDIR, "read-e.desktop"))
print(f"  [+] Created desktop entries")

# 5. AppRun script
apprun_content = """#!/bin/sh
set -e

HERE="$(dirname "$(readlink -f "${0}")")"
export PATH="${HERE}/usr/bin:${PATH}"
export LD_LIBRARY_PATH="${HERE}/usr/lib:${HERE}/usr/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"
export XDG_DATA_DIRS="${HERE}/usr/share:${XDG_DATA_DIRS:-/usr/local/share:/usr/share}"

exec "${HERE}/usr/bin/read-e" "$@"
"""
apprun_file = os.path.join(APPDIR, "AppRun")
with open(apprun_file, "w", encoding="utf-8") as f:
    f.write(apprun_content)
os.chmod(apprun_file, 0o755)
print(f"  [+] Created AppRun entrypoint")

# 6. Ensure appimagetool is available
if not os.path.isfile(TOOL_PATH):
    print("  [+] Downloading appimagetool...")
    url = "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage"
    req = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
    with urllib.request.urlopen(req) as resp, open(TOOL_PATH, 'wb') as out_f:
        shutil.copyfileobj(resp, out_f)
    os.chmod(TOOL_PATH, 0o755)
    print("  [+] Downloaded appimagetool successfully")

# 7. Package AppImage using appimagetool
print(f"  [+] Building AppImage...")
env = os.environ.copy()
env["ARCH"] = ARCH

# Use --appimage-extract-and-run to ensure compatibility on systems without libfuse2 (e.g. Ubuntu 24.04+)
cmd = [TOOL_PATH, "--appimage-extract-and-run", "-n", APPDIR, APPIMAGE_OUT]
res = subprocess.run(cmd, env=env)

if res.returncode != 0:
    print(f"Error: appimagetool failed with code {res.returncode}")
    exit(1)

size_mb = os.path.getsize(APPIMAGE_OUT) / (1024 * 1024)
print(f"==> Successfully built {APPIMAGE_OUT} ({size_mb:.2f} MB)")
