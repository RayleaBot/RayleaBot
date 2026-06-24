.PHONY: doctor check-server-structure check-toolchain

doctor: check-toolchain check-server-structure

check-toolchain:
	python scripts/check-toolchain.py

check-server-structure:
	python scripts/check-server-structure.py
