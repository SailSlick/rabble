#!/usr/bin/env python3
from concurrent import futures
import grpc
import time

from utils.connect import get_service_channel
from utils.logger import get_logger
from utils.users import UsersUtil
from utils.recommenders import RecommendersUtil
from follows.servicer import FollowsServicer
from follows.util import Util

from services.proto import database_pb2_grpc
from services.proto import follows_pb2_grpc
from services.proto import s2s_follow_pb2_grpc
from services.proto import rss_pb2_grpc
from services.proto import approver_pb2_grpc


def main():
    logger = get_logger('follows_service')
    logger.info('Creating server')

    db_env = 'DB_SERVICE_HOST'
    follow_env = 'FOLLOW_ACTIVITY_SERVICE_HOST'
    approver_env = 'APPROVER_SERVICE_HOST'
    rss_env = 'RSS_SERVICE_HOST'

    with get_service_channel(logger, db_env, 1798) as db_chan, \
            get_service_channel(logger, follow_env, 1922) as follow_chan, \
            get_service_channel(logger, approver_env, 2077) as approver_chan, \
            get_service_channel(logger, rss_env, 1973) as rss_chan:

        db_stub = database_pb2_grpc.DatabaseStub(db_chan)
        rss_stub = rss_pb2_grpc.RSSStub(rss_chan)
        follow_stub = s2s_follow_pb2_grpc.S2SFollowStub(follow_chan)
        approver_stub = approver_pb2_grpc.ApproverStub(approver_chan)
        users_util = UsersUtil(logger, db_stub)

        util = Util(logger, db_stub, approver_stub, users_util)
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

        recommender_util = RecommendersUtil(logger, db_stub)
        follow_recommender_stub = recommender_util.get_follow_recommender_stub()

        follows_servicer = FollowsServicer(logger, util, users_util, db_stub,
                                           follow_stub, approver_stub, rss_stub,
                                           follow_recommender_stub)
        follows_pb2_grpc.add_FollowsServicer_to_server(follows_servicer,
                                                       server)

        server.add_insecure_port('0.0.0.0:1641')
        logger.info("Starting follows service on port 1641")
        server.start()
        try:
            while True:
                time.sleep(60 * 60 * 24)  # One day
        except KeyboardInterrupt:
            pass


if __name__ == '__main__':
    main()
