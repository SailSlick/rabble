import os
import sys

from services.proto import database_pb2
from services.proto import general_pb2
from services.proto import recommend_follows_pb2


class ReceiveFollowServicer:

    def __init__(self, logger, util, users_util, database_stub, recommender_stub):
        self._logger = logger
        self._util = util
        self._users_util = users_util
        self._database_stub = database_stub
        self._recommender_stub = recommender_stub
        self._host_name = os.environ.get("HOST_NAME")
        if not self._host_name:
            print("Please set HOST_NAME env variable")
            sys.exit(1)

    def ReceiveFollowRequest(self, request, context):
        resp = general_pb2.GeneralResponse()
        local_user, foreign_user = self._util.validate_and_get_users(resp,
                                                                     request)
        if foreign_user is None or local_user is None:
            return resp

        self._logger.info('User ID %d has requested to follow User ID %d',
                          foreign_user.global_id,
                          local_user.global_id)

        if not local_user.private.value:
            self._logger.info('Accepting follow request')
            self._util.attempt_to_accept(
                local_user, foreign_user, self._host_name, True)

        state = database_pb2.Follow.ACTIVE
        if local_user.private.value:
            self._logger.info('Follow private user: waiting for approval')
            state = database_pb2.Follow.PENDING

        follow_resp = self._util.create_follow_in_db(foreign_user.global_id,
                                                     local_user.global_id,
                                                     state=state)
        if follow_resp.result_type == general_pb2.ResultType.ERROR:
            self._logger.error('Error creating follow: %s', follow_resp.error)
            resp.result_type = general_pb2.ResultType.ERROR
            resp.error = 'Could not add requested follow to database'
            return resp

        if self._recommender_stub is not None:
            req = recommend_follows_pb2.UpdateFollowRecommendationsRequest(
                follower=foreign_user.global_id,
                followed=local_user.global_id,
                following=True)
            self._recommender_stub.UpdateFollowRecommendations(req)

        resp.result_type = general_pb2.ResultType.OK
        return resp
