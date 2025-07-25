FROM pypiserver/pypiserver:latest

# Install additional tools needed for package downloading
RUN apk add --no-cache python3 py3-pip git

# Create directories for packages
RUN mkdir -p /packages

# Copy the population script and requirements
COPY populate_packages.py /populate_packages.py
COPY populate-requirements.txt /populate-requirements.txt
COPY start.sh /start.sh

# Make scripts executable
RUN chmod +x /populate_packages.py /start.sh

# Expose the port that pypiserver runs on
EXPOSE 8098

# Set the working directory
WORKDIR /

# Override the entrypoint to use our custom start script
ENTRYPOINT ["/start.sh"]
