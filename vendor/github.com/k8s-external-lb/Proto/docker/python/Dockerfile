FROM python

RUN pip install grpcio-tools
RUN pip install googleapis-common-protos

CMD ["python", "-m", "grpc_tools.protoc", "-I.", "--python_out=.", "--grpc_python_out=.","external-loadbalancer.proto"]