from services.proto import users_pb2_grpc
from users.login import LoginHandler
from users.create import CreateHandler
from users.update import UpdateHandler
from users.get_css import GetCssHandler
from users.feed_verification import FeedVerificationHandler


class UsersServicer(users_pb2_grpc.UsersServicer):
    def __init__(self, logger, db_stub):
        self._login = LoginHandler(logger, db_stub)
        self._create = CreateHandler(logger, db_stub)
        self._update = UpdateHandler(logger, db_stub)
        self._get_css = GetCssHandler(logger, db_stub)
        self._feed_verification = FeedVerificationHandler(logger, db_stub)

    def Login(self, request, context):
        return self._login.Login(request, context)

    def Create(self, request, context):
        return self._create.Create(request, context)

    def Update(self, request, context):
        return self._update.Update(request, context)

    def GetCss(self, request, context):
        return self._get_css.GetCss(request, context)

    def CreateFeedVerificationHash(self, request, context):
        return self._feed_verification.CreateFeedVerificationHash()
