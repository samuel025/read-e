.PHONY: all build package clean

all: build package

build:
	wails build

package:
	python3 scripts/package-deb.py

clean:
	rm -rf build/bin dist
