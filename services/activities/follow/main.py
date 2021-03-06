#!/usr/bin/env python3
from concurrent import futures
import grpc
import time

from utils.logger import get_logger
from utils.users import UsersUtil
from utils.activities import ActivitiesUtil
from utils.connect import get_future_channel

from activities.follow.servicer import FollowServicer
from services.proto import follows_pb2_grpc
from services.proto import s2s_follow_pb2_grpc
from services.proto import database_pb2_grpc


def main():
    logger = get_logger("s2s_follow_service")
    users_util = UsersUtil(logger, None)
    with get_future_channel(logger, "DB_SERVICE_HOST", 1798) as db_chan, \
            get_future_channel(logger, "FOLLOWS_SERVICE_HOST", 1641) as logger_chan:
        db_stub = database_pb2_grpc.DatabaseStub(db_chan)
        activ_util = ActivitiesUtil(logger, db_stub)
        follows_service = follows_pb2_grpc.FollowsStub(logger_chan)
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        s2s_follow_pb2_grpc.add_S2SFollowServicer_to_server(
            FollowServicer(logger, users_util, activ_util,
                           follows_service, db_stub),
            server
        )
        server.add_insecure_port('0.0.0.0:1922')
        logger.info("Starting s2s follow server on port 1922")
        server.start()
        while True:
            time.sleep(60 * 60 * 24)  # One day


if __name__ == '__main__':
    main()
