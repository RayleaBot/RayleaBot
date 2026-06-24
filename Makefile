.PHONY: doctor check-toolchain

doctor: check-toolchain

check-toolchain:
	python scripts/check-toolchain.py
