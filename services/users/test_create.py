import unittest
from unittest.mock import Mock

from users.create import CreateHandler
from services.proto import database_pb2
from services.proto import users_pb2
from services.proto import general_pb2


class MockDBStub:
    def __init__(self):
        self.Users = Mock()


class CreateHandlerTest(unittest.TestCase):
    def setUp(self):
        self.db_stub = MockDBStub()
        self.create_handler = CreateHandler(Mock(), self.db_stub)

    def _make_request(self, handle):
        return users_pb2.CreateUserRequest(
            handle=handle,
            password="123",
            display_name="myname",
            bio="mybio",
        )

    def test_handle_error(self):
        req = self._make_request("CianLR")
        err = "MockError"
        self.db_stub.Users.return_value = database_pb2.UsersResponse(
            result_type=general_pb2.ResultType.ERROR,
            error=err,
        )
        resp = self.create_handler.Create(req, None)
        self.assertEqual(resp.result_type, general_pb2.ResultType.ERROR)
        self.assertEqual(resp.error, err)

    def test_send_db_request(self):
        req = self._make_request("CianLR")
        self.db_stub.Users.return_value = database_pb2.UsersResponse(
            result_type=general_pb2.ResultType.OK,
            global_id=2
        )
        resp = self.create_handler.Create(req, None)
        self.assertEqual(resp.result_type, general_pb2.ResultType.OK)
        self.assertEqual(resp.global_id, 2)
        self.assertNotEqual(self.db_stub.Users.call_args, None)
        db_req = self.db_stub.Users.call_args[0][0]
        self.assertEqual(db_req.entry.handle, "CianLR")
