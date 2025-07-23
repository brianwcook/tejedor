FROM registry.access.redhat.com/ubi9/ubi
RUN dnf update -y && dnf install -y python3 python3-pip git gcc python3-devel --allowerasing && dnf clean all
RUN mkdir -p /opt/pypi-server/packages /opt/pypi-server/private
RUN pip3 install pypiserver
RUN cat > /opt/pypi-server/populate_packages.py << "EOF"
RUN cat > /opt/pypi-server/populate_packages.py << "EOF"
#!/usr/bin/env python3
import subprocess
from pathlib import Path

def download_package(package_name):
    packages_dir = Path("/opt/pypi-server/packages")
    packages_dir.mkdir(exist_ok=True)
    cmd = ["pip3", "download", "--no-deps", "--dest", str(packages_dir), package_name]
    try:
        subprocess.run(cmd, check=True, capture_output=True)
        print(f"Successfully downloaded {package_name}")
        return True
    except subprocess.CalledProcessError as e:
        print(f"Failed to download {package_name}: {e}")
        return False

def main():
    packages = ["six", "flask", "requests", "click", "jinja2", "werkzeug", "markupsafe", "itsdangerous", "blinker"]
    print("Downloading packages from public PyPI...")
    for package in packages:
        download_package(package)
    print("Package population complete!")

if __name__ == "__main__":
    main()
EOF
RUN chmod +x /opt/pypi-server/populate_packages.py
RUN cat > /opt/pypi-server/start.sh << "EOF"
#!/bin/bash
set -e
echo "Starting PyPI server setup..."
python3 /opt/pypi-server/populate_packages.py
echo "Starting PyPI server on port 8080..."
cd /opt/pypi-server
exec pypi-server -p 8080 -a . packages
EOF
RUN chmod +x /opt/pypi-server/start.sh
EXPOSE 8080
WORKDIR /opt/pypi-server
CMD ["./start.sh"]
