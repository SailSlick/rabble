from actor_servicer import ActorsServicer
from collection_servicer import CollectionServicer
from services.proto import actors_pb2_grpc


class Servicer(actors_pb2_grpc.ActorsServicer):

    def __init__(self, logger, users_util, activ_util, db_stub, follows_stub):
        self._logger = logger
        self._users_util = users_util
        self._activ_util = activ_util
        self._db_stub = db_stub
        self._follows_stub = follows_stub

        actor_servicer = ActorsServicer(
            db_stub, logger, users_util, activ_util)
        self.Get = actor_servicer.Get
        self.GetArticle = actor_servicer.GetArticle

        collection_servicer = CollectionServicer(
            logger, users_util, activ_util, db_stub, follows_stub)
        self.GetFollowing = collection_servicer.GetFollowing
        self.GetFollowers = collection_servicer.GetFollowers
