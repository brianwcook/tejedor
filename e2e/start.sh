#!/bin/bash
set -e

echo "Starting PyPI server setup..."

# Populate packages
echo "Downloading packages from public PyPI..."
python3 populate_packages.py

# Start PyPI server using the official pypiserver
echo "Starting PyPI server on port 8098..."
pypi-server run -p 8098 -i 0.0.0.0 /packages