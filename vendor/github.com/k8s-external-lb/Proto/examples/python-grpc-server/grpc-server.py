from concurrent import futures
import time
import grpc

import external_loadbalancer_pb2
import external_loadbalancer_pb2_grpc

_ONE_DAY_IN_SECONDS = 60 * 60 * 24

class ExternalLoadbalancer(external_loadbalancer_pb2_grpc.ExternalLoadBalancerServicer):

    def __init__(self):
        self.ipAddr = 0
        self.mapAddr = {}

    def Create(self, request, context):
        self.ipAddr = self.ipAddr + 1
        self.mapAddr[request.FarmName] = "10.0.0.{1}".format(self.ipAddr)
        return external_loadbalancer_pb2.Result(FarmAddress= "10.0.0.{1}".format(self.ipAddr))

    def Update(self, request, context):
        if not request.FarmName in self.mapAddr:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details('Farm not found')
            return external_loadbalancer_pb2.Response()
        

    def Delete(self, request, context):
        del self.mapAddr[request.FarmName]
        return external_loadbalancer_pb2.Result()
    
    def NodesChange(self, request, context):
        return external_loadbalancer_pb2.Result()



def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    external_loadbalancer_pb2_grpc.add_ExternalLoadBalancerServicer_to_server(ExternalLoadbalancer(), server)
    server.add_insecure_port('0.0.0.0:8050')
    server.start()
    try:
        while True:
            time.sleep(_ONE_DAY_IN_SECONDS)
    except KeyboardInterrupt:
        server.stop(0)


if __name__ == '__main__':
    serve()