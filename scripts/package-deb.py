#!/usr/bin/env python3
import os
import shutil
import subprocess
import cairo
import gi

gi.require_version('Rsvg', '2.0')
from gi.repository import Rsvg

ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
BUILD_BIN = os.path.join(ROOT_DIR, "build", "bin", "read-e")
SVG_ICON = os.path.join(ROOT_DIR, "build", "appicon.svg")
DIST_DIR = os.path.join(ROOT_DIR, "dist")
PKG_DIR = os.path.join(DIST_DIR, "deb-pkg")

VERSION_FILE = os.path.join(ROOT_DIR, "VERSION")
if os.path.isfile(VERSION_FILE):
    with open(VERSION_FILE, "r") as f:
        VERSION = f.read().strip()
else:
    VERSION = "1.0.1"

ARCH = "amd64"
DEB_NAME = f"read-e_{VERSION}_{ARCH}.deb"
DEB_PATH = os.path.join(DIST_DIR, DEB_NAME)

print(f"==> Packaging read e v{VERSION} for Debian/Ubuntu ({ARCH})...")

# Check binary exists
if not os.path.isfile(BUILD_BIN):
    print(f"Error: Binary not found at {BUILD_BIN}. Run 'wails build' first.")
    exit(1)

# Clean previous build
if os.path.exists(PKG_DIR):
    shutil.rmtree(PKG_DIR)
os.makedirs(DIST_DIR, exist_ok=True)

# Create directory hierarchy
bin_dir = os.path.join(PKG_DIR, "usr", "bin")
apps_dir = os.path.join(PKG_DIR, "usr", "share", "applications")
pixmaps_dir = os.path.join(PKG_DIR, "usr", "share", "pixmaps")
hicolor_dir = os.path.join(PKG_DIR, "usr", "share", "icons", "hicolor")
debian_dir = os.path.join(PKG_DIR, "DEBIAN")

os.makedirs(bin_dir, exist_ok=True)
os.makedirs(apps_dir, exist_ok=True)
os.makedirs(pixmaps_dir, exist_ok=True)
os.makedirs(debian_dir, exist_ok=True)

# 1. Copy Binary
dest_bin = os.path.join(bin_dir, "read-e")
shutil.copy2(BUILD_BIN, dest_bin)
os.chmod(dest_bin, 0o755)
print(f"  [+] Installed binary to /usr/bin/read-e")

# 2. Scalable SVG Icon
scalable_dir = os.path.join(hicolor_dir, "scalable", "apps")
os.makedirs(scalable_dir, exist_ok=True)
shutil.copy2(SVG_ICON, os.path.join(scalable_dir, "read-e.svg"))
print(f"  [+] Installed scalable SVG icon")

# 3. Render PNG icon sizes
handle = Rsvg.Handle.new_from_file(SVG_ICON)
dim = handle.get_dimensions()

sizes = [16, 32, 48, 64, 128, 256, 512]
for sz in sizes:
    sz_dir = os.path.join(hicolor_dir, f"{sz}x{sz}", "apps")
    os.makedirs(sz_dir, exist_ok=True)
    out_path = os.path.join(sz_dir, "read-e.png")

    surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, sz, sz)
    ctx = cairo.Context(surface)
    ctx.scale(sz / dim.width, sz / dim.height)
    handle.render_cairo(ctx)
    surface.write_to_png(out_path)

# Legacy pixmap icon (128x128)
shutil.copy2(os.path.join(hicolor_dir, "128x128", "apps", "read-e.png"), os.path.join(pixmaps_dir, "read-e.png"))
print(f"  [+] Generated PNG icons: {sizes}")

# 4. Desktop Entry
desktop_content = """[Desktop Entry]
Type=Application
Name=read e
GenericName=E-Book Reader
Comment=Modern EPUB and PDF reader with reading insights
Exec=/usr/bin/read-e %U
Icon=read-e
Terminal=false
Categories=Office;Viewer;Literature;
MimeType=application/epub+zip;application/pdf;
StartupWMClass=read-e
Keywords=read;epub;pdf;ebook;reader;books;insights;
"""
desktop_file = os.path.join(apps_dir, "read-e.desktop")
with open(desktop_file, "w", encoding="utf-8") as f:
    f.write(desktop_content)
os.chmod(desktop_file, 0o644)
print(f"  [+] Created desktop entry")

# 5. Calculate Installed-Size in KB
total_size = 0
for dirpath, dirnames, filenames in os.walk(os.path.join(PKG_DIR, "usr")):
    for f in filenames:
        fp = os.path.join(dirpath, f)
        total_size += os.path.getsize(fp)
installed_size_kb = (total_size + 1023) // 1024

# 6. Control File
control_content = f"""Package: read-e
Version: {VERSION}
Section: utils
Priority: optional
Architecture: {ARCH}
Maintainer: Samuel <samuel@localhost>
Installed-Size: {installed_size_kb}
Depends: libc6, libgtk-3-0 | libgtk-3-0t64, libwebkit2gtk-4.1-0
Description: read e - Modern EPUB & PDF Reader with Reading Insights
 Fast, elegant desktop EPUB and PDF reader built with Wails and Svelte.
 Features include smooth pagination, search, customizable themes and fonts,
 bookmarks, highlights, and a Reading Insights dashboard tracking reading
 streaks and progress.
"""
with open(os.path.join(debian_dir, "control"), "w", encoding="utf-8") as f:
    f.write(control_content)
print(f"  [+] Created DEBIAN/control (Installed-Size: {installed_size_kb} KB)")

# 7. Maintainer Scripts (postinst & postrm)
postinst_content = """#!/bin/sh
set -e

if [ "$1" = "configure" ]; then
    if [ -x /usr/bin/update-desktop-database ]; then
        update-desktop-database -q || true
    fi
    if [ -x /usr/bin/gtk-update-icon-cache ]; then
        gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
    fi
fi
exit 0
"""
postinst_file = os.path.join(debian_dir, "postinst")
with open(postinst_file, "w", encoding="utf-8") as f:
    f.write(postinst_content)
os.chmod(postinst_file, 0o755)

postrm_content = """#!/bin/sh
set -e

if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
    if [ -x /usr/bin/update-desktop-database ]; then
        update-desktop-database -q || true
    fi
    if [ -x /usr/bin/gtk-update-icon-cache ]; then
        gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
    fi
fi
exit 0
"""
postrm_file = os.path.join(debian_dir, "postrm")
with open(postrm_file, "w", encoding="utf-8") as f:
    f.write(postrm_content)
os.chmod(postrm_file, 0o755)

# 8. Build .deb package
cmd = ["dpkg-deb", "--build", "--root-owner-group", PKG_DIR, DEB_PATH]
result = subprocess.run(cmd, capture_output=True, text=True)
if result.returncode != 0:
    print("Error building debian package:")
    print(result.stderr)
    exit(1)

# Clean up staging directory
if os.path.exists(PKG_DIR):
    shutil.rmtree(PKG_DIR)

deb_size_mb = os.path.getsize(DEB_PATH) / (1024 * 1024)
print(f"==> Successfully built {DEB_PATH} ({deb_size_mb:.2f} MB)")
