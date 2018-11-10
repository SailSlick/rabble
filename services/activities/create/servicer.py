from send_create_servicer import SendCreateServicer
from receive_create_servicer import ReceiveCreateServicer
from proto import create_pb2_grpc


class CreateServicer(create_pb2_grpc.CreateServicer):

    def __init__(self, db_stub, logger):
        self._logger = logger
        self._db_stub = db_stub

        send_create_servicer = SendCreateServicer(db_stub, logger)
        self.SendCreate = send_create_servicer.SendCreate
        receive_create_servicer = ReceiveCreateServicer(db_stub, logger)
        self.ReceiveCreate = receive_create_servicer.ReceiveCreate
