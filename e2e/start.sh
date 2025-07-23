#!/bin/bash
set -e

echo "Starting PyPI server setup..."

# Populate packages
echo "Downloading packages from public PyPI..."
python3 populate_packages.py

# Start PyPI server
echo "Starting PyPI server on port 8080..."
pypi-server run -p 8080 packages 