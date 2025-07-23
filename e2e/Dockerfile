FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install necessary packages
RUN microdnf update -y && \
    microdnf install -y python3 python3-pip git gcc python3-devel && \
    microdnf clean all

# Create directories
RUN mkdir -p /opt/pypi-server/packages /opt/pypi-server/private

# Install PyPI server
RUN pip3 install pypiserver

# Create a script to populate packages from public PyPI
RUN cat > /opt/pypi-server/populate_packages.py << 'EOF'
#!/usr/bin/env python3
import os
import subprocess
import sys
import tempfile
import shutil
from pathlib import Path

def download_package(package_name, version=None):
    """Download a package from public PyPI to local directory."""
    packages_dir = Path("/opt/pypi-server/packages")
    packages_dir.mkdir(exist_ok=True)
    
    # Use pip to download the package
    cmd = [
        "pip3", "download", 
        "--no-deps",  # Don't download dependencies
        "--dest", str(packages_dir),
        package_name
    ]
    
    if version:
        cmd.append(f"{package_name}=={version}")
    
    try:
        subprocess.run(cmd, check=True, capture_output=True)
        print(f"Successfully downloaded {package_name}")
        return True
    except subprocess.CalledProcessError as e:
        print(f"Failed to download {package_name}: {e}")
        return False

def main():
    # List of packages to download (mix of source-only and wheel packages)
    packages = [
        "six",           # Source distribution
        "flask",         # Has both source and wheel
        "requests",      # Has both source and wheel
        "click",         # Has both source and wheel
        "jinja2",        # Has both source and wheel
        "werkzeug",      # Has both source and wheel
        "markupsafe",    # Has both source and wheel
        "itsdangerous",  # Has both source and wheel
        "blinker",       # Has both source and wheel
    ]
    
    print("Downloading packages from public PyPI...")
    for package in packages:
        download_package(package)
    
    print("Package population complete!")

if __name__ == "__main__":
    main()
EOF

RUN chmod +x /opt/pypi-server/populate_packages.py

# Create startup script
RUN cat > /opt/pypi-server/start.sh << 'EOF'
#!/bin/bash
set -e

echo "Starting PyPI server setup..."

# Populate packages from public PyPI
python3 /opt/pypi-server/populate_packages.py

# Start PyPI server
echo "Starting PyPI server on port 8080..."
cd /opt/pypi-server
exec pypi-server -p 8080 -a . packages
EOF

RUN chmod +x /opt/pypi-server/start.sh

# Expose port
EXPOSE 8080

# Set working directory
WORKDIR /opt/pypi-server

# Start the server
CMD ["./start.sh"] 