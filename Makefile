.PHONY: all build package package-deb package-appimage clean

all: build package

build:
	wails build

package: package-deb package-appimage

package-deb:
	python3 scripts/package-deb.py

package-appimage:
	python3 scripts/package-appimage.py

clean:
	rm -rf build/bin dist

