FROM registry.access.redhat.com/ubi9/ubi
RUN dnf update -y && dnf install -y python3 python3-pip git gcc python3-devel --allowerasing && dnf clean all
RUN mkdir -p /opt/pypi-server/packages /opt/pypi-server/private
RUN pip3 install pypiserver
COPY populate_packages.py /opt/pypi-server/populate_packages.py
COPY populate-requirements.txt /opt/pypi-server/populate-requirements.txt
COPY start.sh /opt/pypi-server/start.sh
RUN chmod +x /opt/pypi-server/populate_packages.py /opt/pypi-server/start.sh
EXPOSE 8080
WORKDIR /opt/pypi-server
CMD ["./start.sh"]
