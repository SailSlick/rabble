import os

from surprise_recommender import SurpriseRecommender
from noop_recommender import NoopRecommender
from cn_recommender import CNRecommender
from gd_recommender import GraphDistanceRecommender

from services.proto import follows_pb2_grpc
from services.proto import database_pb2
from services.proto import recommend_follows_pb2


class FollowRecommendationsServicer(follows_pb2_grpc.FollowsServicer):

    RECOMMENDERS = {
        'surprise': SurpriseRecommender,
        'none': NoopRecommender,
        'cn': CNRecommender,
        'graphdist': GraphDistanceRecommender,
    }
    DEFAULT_RECOMMENDER = 'none'
    ENV_VAR = 'FOLLOW_RECOMMENDER_METHOD'
    DEFAULT_IMAGE = "https://qph.fs.quoracdn.net/main-qimg-8aff684700be1b8c47fa370b6ad9ca13.webp"

    def __init__(self, logger, users_util, db_stub):
        self._logger = logger
        self._users_util = users_util
        self._db_stub = db_stub
        self._recommender_util = RecommendersUtil(
            logger, db, DEFAULT_RECOMMENDER, ENV_VAR, RECOMMENDERS)

        # self.active_recommenders contains one or more recommender system
        # objects (out of the constructors in self.RECOMMENDERS).
        self.active_recommenders = self._recommender_util._get_active_recommenders()

    def _get_recommendations(self, user_id):
        '''Get recommendations for users for the given user_id to follow, using
        the one or more systems in self.active_recommenders. Could return empty
        list if there are no good recommendations.'''
        # TODO(iandioch): Allow for combining the results of multiple systems
        # in a smarter way than just concatenation.
        for r in self.active_recommenders:
            yield from r.get_recommendations(user_id)

    def GetFollowRecommendations(self, request, context):
        self._logger.debug('GetFollowRecommendations, username = %s',
                           request.username)

        resp = recommend_follows_pb2.FollowRecommendationResponse()

        handle, host = self._users_util.parse_username(request.username)
        if not (host is None or host == ""):
            resp.result_type = \
                recommend_follows_pb2.FollowRecommendationResponse.ERROR
            resp.error = "Can only give recommendations for local users."
            return resp

        user = self._users_util.get_user_from_db(
            handle=handle, host_is_null=True)
        if user is None:
            resp.result_type = \
                recommend_follows_pb2.FollowRecommendationResponse.ERROR
            resp.error = "Could not find the given username."
            return resp

        resp.result_type = \
            recommend_follows_pb2.FollowRecommendationResponse.OK

        # Get the recommendations and package them into proto.
        for p in self._get_recommendations(user.global_id):
            a = self._users_util.get_or_create_user_from_db(global_id=p[0])
            user_obj = resp.results.add()
            user_obj.handle = a.handle
            user_obj.host = a.host
            user_obj.display_name = a.display_name
            user_obj.bio = a.bio
            user_obj.image = self.DEFAULT_IMAGE
            user_obj.global_id = a.global_id
        return resp
