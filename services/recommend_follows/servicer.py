import os

from surprise_recommender import SurpriseRecommender

from services.proto import follows_pb2_grpc
from services.proto import database_pb2
from services.proto import recommend_follows_pb2


class FollowRecommendationsServicer(follows_pb2_grpc.FollowsServicer):

    RECOMMENDERS = {
        'surprise': SurpriseRecommender,
    }
    DEFAULT_RECOMMENDER = 'surprise'
    ENV_VAR = 'follow_recommender'

    def __init__(self, logger, users_util, db_stub):
        self._logger = logger
        self._users_util = users_util
        self._db_stub = db_stub

        self.active_recommenders = self._get_active_recommenders()

    def _get_active_recommenders(self):
        # TODO(iandioch): Explain this func.
        keys = [self.DEFAULT_RECOMMENDER]
        if self.ENV_VAR not in os.environ:
            self._logger.warning('No value set for "follow_recommender" ' + 
                                 'environment variable, using default of ' +
                                 '"{}".'.format(self.DEFAULT_RECOMMENDER))
        else:
            keys = set()
            for a in os.environ[self.ENV_VAR].split(','):
                if a in self.RECOMMENDERS:
                    keys.append(a)
                else:
                    self._logger.warning('Follow recommender {} '.format(a) +
                                         'requested, but no such system found. '
                                         'Skipping.')
            if len(keys) == 0:
                self._logger.warning('No valid values given for follow '
                                     'recommender, using default of ' +
                                     '"{}".'.format(self.DEFAULT_RECOMMENDER))
                keys = [self.DEFAULT_RECOMMENDER]

        # At this point, keys[] should contain either the default system, or
        # a list of user-chosen ones.
        recommenders = []
        for k in keys:
            constructor = self.RECOMMENDERS[k]
            r = constructor(self._logger, self._users_util, self._db_stub)
            recommenders.append(r)
        return recommenders


    def _get_recommendations(self, user_id):
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

        user = self._users_util.get_user_from_db(handle=handle, host=None)
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
        return resp
