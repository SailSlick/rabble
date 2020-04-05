#!/usr/bin/env python3
from concurrent import futures
import grpc
import time

from services.proto import database_pb2_grpc
from services.proto import undo_pb2_grpc
from utils.activities import ActivitiesUtil
from utils.connect import get_service_channel
from utils.logger import get_logger
from utils.users import UsersUtil
from servicer import S2SUndoServicer


def get_db_stub(logger):
    chan = get_service_channel(logger, "DB_SERVICE_HOST", 1798)
    return database_pb2_grpc.DatabaseStub(chan)


def main():
    logger = get_logger("undo_service")
    db_stub = get_db_stub(logger)
    activ_util = ActivitiesUtil(logger, db_stub)
    users_util = UsersUtil(logger, db_stub)
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    undo_pb2_grpc.add_S2SUndoServicer_to_server(
        S2SUndoServicer(logger, db_stub, activ_util, users_util),
        server
    )
    server.add_insecure_port("0.0.0.0:1608")
    logger.info("Starting Undo service on port 1608")
    server.start()
    try:
        while True:
            time.sleep(60 * 60 * 24)  # One day
    except KeyboardInterrupt:
        pass


if __name__ == '__main__':
    main()
