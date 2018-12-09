from follow_servicer import FollowDatabaseServicer
from posts_servicer import PostsDatabaseServicer
from users_servicer import UsersDatabaseServicer
from like_servicer import LikeDatabaseServicer

from services.proto import database_pb2_grpc


class DatabaseServicer(database_pb2_grpc.DatabaseServicer):

    def __init__(self, db, logger):
        self._db = db
        self._logger = logger

        posts_servicer = PostsDatabaseServicer(db, logger)
        self.Posts = posts_servicer.Posts
        self.InstanceFeed = posts_servicer.InstanceFeed
        users_servicer = UsersDatabaseServicer(db, logger)
        self.Users = users_servicer.Users
        self.PendingFollows = users_servicer.PendingFollows
        follow_servicer = FollowDatabaseServicer(db, logger)
        self.Follow = follow_servicer.Follow
        like_servicer = LikeDatabaseServicer(db, logger)
        self.AddLike = like_servicer.AddLike
        self.LikedCollection = like_servicer.LikedCollection
        self.LikesCollection = like_servicer.LikesCollection

