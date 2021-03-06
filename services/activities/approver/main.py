#!/usr/bin/env python3
from concurrent import futures

import grpc
import time
from utils.activities import ActivitiesUtil
from utils.connect import get_service_channel
from utils.logger import get_logger
from utils.users import UsersUtil
from activities.approver.servicer import ApproverServicer
from services.proto import database_pb2_grpc
from services.proto import approver_pb2_grpc


def main():
    logger = get_logger("create_service")
    logger.info("Creating db connection")

    with get_service_channel(logger, "DB_SERVICE_HOST", 1798) as db_chan:
        db_stub = database_pb2_grpc.DatabaseStub(db_chan)
        users_util = UsersUtil(logger, db_stub)
        activ_util = ActivitiesUtil(logger, db_stub)

        server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        approver_pb2_grpc.add_ApproverServicer_to_server(
            ApproverServicer(logger, db_stub, activ_util, users_util),
            server,
        )

        server.add_insecure_port('0.0.0.0:2077')
        logger.info("Starting approver service on port 2077")
        server.start()
        while True:
            time.sleep(60 * 60 * 24)  # One day


if __name__ == '__main__':
    main()
