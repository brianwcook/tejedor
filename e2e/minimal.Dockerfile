FROM registry.access.redhat.com/ubi9/ubi
RUN dnf install -y python3 python3-pip
RUN pip3 install pypiserver
EXPOSE 8080
CMD ["pypi-server", "-p", "8080", "-a", ".", "/tmp"]
