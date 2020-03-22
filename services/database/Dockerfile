FROM rabblenetwork/rabble_base

ARG DBPATH
ENV DBPATH=$DBPATH

CMD python3 -u -B /repo/build_out/database/main.py -v --schema /repo/build_out/database/rabble_schema.sql --db_path $DBPATH
