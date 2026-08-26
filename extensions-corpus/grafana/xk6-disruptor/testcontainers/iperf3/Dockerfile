FROM debian:trixie@sha256:fd8f5a1df07b5195613e4b9a0b6a947d3772a151b81975db27d47f093f60c6e6

RUN apt update && apt install -y iperf3

ENTRYPOINT [ "iperf3" ]
