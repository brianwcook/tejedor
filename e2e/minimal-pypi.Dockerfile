FROM python:3.11-alpine

# Install pypiserver
RUN pip install pypiserver

# Create directories
RUN mkdir -p /opt/pypi-server/packages

# Create a simple test package
RUN mkdir -p /tmp/test-package && \
    cd /tmp/test-package && \
    echo 'from setuptools import setup; setup(name="testpackage", version="1.0.0")' > setup.py && \
    python setup.py sdist && \
    mv dist/* /opt/pypi-server/packages/ && \
    rm -rf /tmp/test-package

# Create a simple start script
RUN echo '#!/bin/sh' > /opt/pypi-server/start.sh && \
    echo 'cd /opt/pypi-server' >> /opt/pypi-server/start.sh && \
    echo 'pypi-server run -p 8080 packages' >> /opt/pypi-server/start.sh && \
    chmod +x /opt/pypi-server/start.sh

WORKDIR /opt/pypi-server

EXPOSE 8080

CMD ["./start.sh"] 