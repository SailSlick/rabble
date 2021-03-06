import os
import sys

from services.proto import database_pb2
from services.proto import s2s_follow_pb2
from services.proto import recommend_follows_pb2
from services.proto import general_pb2


class SendFollowServicer:

    def __init__(self, logger, util, users_util,
                 database_stub, follow_activity_stub, recommender_stub):
        host_name = os.environ.get("HOST_NAME")
        if not host_name:
            print("Please set HOST_NAME env variable")
            sys.exit(1)
        self._host_name = host_name
        self._logger = logger
        self._util = util
        self._users_util = users_util
        self._database_stub = database_stub
        self._follow_activity_stub = follow_activity_stub
        self._recommender_stub = recommender_stub

    def _send_s2s(self, from_handle, to_handle, to_host):
        local_user = s2s_follow_pb2.FollowActivityUser()
        local_user.handle = from_handle
        local_user.host = self._host_name

        foreign_user = s2s_follow_pb2.FollowActivityUser()
        foreign_user.handle = to_handle
        foreign_user.host = to_host

        s2s_follow = s2s_follow_pb2.FollowDetails()
        s2s_follow.follower.handle = from_handle
        s2s_follow.follower.host = self._host_name
        s2s_follow.followed.handle = to_handle
        s2s_follow.followed.host = to_host
        resp = self._follow_activity_stub.SendFollowActivity(s2s_follow)
        if resp.result_type == general_pb2.ResultType.ERROR:
            return resp.error
        return None

    def _add_follow(self, resp, follower_id, followed_id, is_private_followed, is_foreign):
        state = database_pb2.Follow.ACTIVE
        if is_foreign or is_private_followed:
            self._logger.info('PENDING follow request: waiting for approval')
            state = database_pb2.Follow.PENDING

        follow_resp = self._util.create_follow_in_db(follower_id, followed_id,
                                                     state=state)
        if follow_resp.result_type == general_pb2.ResultType.ERROR:
            self._logger.error('Error creating follow: %s', follow_resp.error)
            resp.result_type = general_pb2.ResultType.ERROR
            resp.error = 'Could not add requested follow to database'
            return resp.error

    def _roll_back_follow(self, follower_id, followed_id, user_created):
        self._logger.info("Rolling back follow of %d", followed_id)
        self._util.delete_follow_in_db(follower_id, followed_id)
        if user_created:
            self._users_util.delete_user_from_db(followed_id)

    def SendFollowRequest(self, request, context):
        resp = general_pb2.GeneralResponse()
        self._logger.info('Sending follow request.')

        from_handle, from_instance = self._users_util.parse_username(
            request.follower)
        to_handle, to_instance = \
            self._users_util.parse_username(request.followed)
        self._logger.info('%s@%s has requested to follow %s@%s.',
                          from_handle,
                          from_instance,
                          to_handle,
                          to_instance)
        if to_instance is None and to_handle is None:
            resp.result_type = general_pb2.ResultType.ERROR
            resp.error = 'Could not parse followed username'
            return resp

        # Get user IDs for follow.
        follower_entry = self._users_util.get_or_create_user_from_db(
            handle=from_handle, host=from_instance,
            host_is_null=(from_instance is None))
        if follower_entry is None:
            error = 'Could not find or create user {}@{}'.format(from_handle,
                                                                 from_instance)
            self._logger.error(error)
            resp.result_type = general_pb2.ResultType.ERROR
            resp.error = error
            return resp

        is_local = to_instance is None
        created_user = False
        followed_entry = self._users_util.get_user_from_db(
            handle=to_handle, host=to_instance,
            host_is_null=is_local)
        if not is_local:
            created_user = True
            fu_host, _, fu_bio = self._users_util.get_actor_details(
                to_handle, to_instance)
            if fu_host is None:
                resp.result_type = general_pb2.ResultType.ERROR
                resp.error = "Invalid foreign user to follow"
                return resp
            followed_entry = self._users_util.get_or_create_user_from_db(
                handle=to_handle, host=to_instance, bio=fu_bio)

        if followed_entry is None:
            error = 'Could not find or create user {}@{}'.format(to_handle,
                                                                 to_instance)
            self._logger.error(error)
            resp.result_type = general_pb2.ResultType.ERROR
            resp.error = error
            return resp
        self._logger.info('User ID %d has requested to follow User ID %d',
                          follower_entry.global_id,
                          followed_entry.global_id)

        err = self._add_follow(resp,
                               follower_entry.global_id,
                               followed_entry.global_id,
                               followed_entry.private.value,
                               not is_local)
        if err is not None:
            return resp

        if not is_local:
            err = self._send_s2s(from_handle, to_handle, to_instance)
            if err is not None:
                self._logger.error("Error from s2sFollow: %s", err)
                self._roll_back_follow(follower_entry.global_id,
                                       followed_entry.global_id,
                                       created_user)
                resp.result_type = general_pb2.ResultType.ERROR
                resp.error = err
                return resp

        if self._recommender_stub is not None:
            req = recommend_follows_pb2.UpdateFollowRecommendationsRequest(
                follower=follower_entry.global_id,
                followed=followed_entry.global_id,
                following=True)
            self._recommender_stub.UpdateFollowRecommendations(req)

        resp.result_type = general_pb2.ResultType.OK
        return resp
