FROM rabblenetwork/rabble_base

RUN apk add --no-cache libffi-dev
RUN apk add --update openssl
RUN pip3 install bcrypt

CMD ["python3", "-u", "-B", "/repo/build_out/users/main.py", "-v"]
