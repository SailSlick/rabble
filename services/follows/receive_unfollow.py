import os
import sys

from services.proto import database_pb2
from services.proto import recommend_follows_pb2
from services.proto import general_pb2


class ReceiveUnfollowServicer:

    def __init__(self, logger, util, users_util, database_stub,
                 recommender_stub):
        self._logger = logger
        self._util = util
        self._users_util = users_util
        self._database_stub = database_stub
        self._recommender_stub = recommender_stub
        self._host_name = os.environ.get("HOST_NAME")
        if not self._host_name:
            print("Please set HOST_NAME env variable")
            sys.exit(1)

    def ReceiveUnfollow(self, request, context):
        resp = general_pb2.GeneralResponse()
        local_user, foreign_user = self._util.validate_and_get_users(resp,
                                                                     request)
        if foreign_user is None or local_user is None:
            self._logger.info('Error receiving unfollow: %s', resp.error)
            return resp

        self._logger.info('User ID %d is unfollowing User ID %d',
                          foreign_user.global_id,
                          local_user.global_id)

        follow_resp = self._util.delete_follow_in_db(foreign_user.global_id,
                                                     local_user.global_id)

        if follow_resp.result_type == general_pb2.ResultType.ERROR:
            self._logger.error('Error deleting follow: %s', follow_resp.error)
            resp.result_type = general_pb2.ResultType.ERROR
            resp.error = 'Could not delete requested follow from database'
            return resp

        if self._recommender_stub is not None:
            req = recommend_follows_pb2.UpdateFollowRecommendationsRequest(
                follower=foreign_user.global_id,
                followed=local_user.global_id,
                following=False)
            self._recommender_stub.UpdateFollowRecommendations(req)

        resp.result_type = general_pb2.ResultType.OK
        return resp
