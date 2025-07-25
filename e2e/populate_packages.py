#!/usr/bin/env python3
import subprocess
from pathlib import Path

def download_package(package_spec):
    packages_dir = Path("/packages")
    packages_dir.mkdir(exist_ok=True)
    
    # Use pip3 for Alpine Linux
    cmd = ["pip3", "download", "--no-deps", "--dest", str(packages_dir), package_spec]
    try:
        subprocess.run(cmd, check=True, capture_output=True)
        print(f"Successfully downloaded {package_spec}")
        return True
    except subprocess.CalledProcessError as e:
        print(f"Failed to download {package_spec}: {e}")
        return False

def read_requirements_file(requirements_path):
    """Read packages from requirements file, skipping comments and empty lines"""
    packages = []
    with open(requirements_path, 'r') as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith('#'):
                packages.append(line)
    return packages

def main():
    requirements_file = Path("/populate-requirements.txt")
    
    if not requirements_file.exists():
        print(f"Error: Requirements file not found at {requirements_file}")
        print("Make sure populate-requirements.txt is copied to the container")
        return
    
    packages = read_requirements_file(requirements_file)
    
    if not packages:
        print("No packages found in requirements file")
        return
    
    print("Downloading packages from public PyPI...")
    for package_spec in packages:
        download_package(package_spec)
    print("Package population complete!")

if __name__ == "__main__":
    main() 