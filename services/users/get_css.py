from services.proto import database_pb2
from services.proto import users_pb2
from services.proto import general_pb2


class GetCssHandler:
    def __init__(self, logger, db_stub):
        self._logger = logger
        self._db = db_stub

    def GetCss(self, request, context):
        self._logger.info("Request to get the CSS for user id %s",
                          request.user_id)
        resp = self._db.Users(database_pb2.UsersRequest(
            request_type=database_pb2.RequestType.FIND,
            match=database_pb2.UsersEntry(
                global_id=request.user_id,
            )
        ))
        if resp.result_type != general_pb2.ResultType.OK:
            self._logger.error("Error getting CSS: %s", resp.error)
            return users_pb2.GetCssResponse(
                result=general_pb2.ResultType.ERROR,
                error=resp.error,
            )
        elif len(resp.results) != 1:
            self._logger.error(
                "Got wrong number of results, expected 1 got %d",
                len(resp.results))
            return users_pb2.GetCssResponse(
                result=general_pb2.ResultType.ERROR,
                error="Got wrong number of results",
            )
        return users_pb2.GetCssResponse(
            result=general_pb2.ResultType.OK,
            css=resp.results[0].custom_css,
        )
