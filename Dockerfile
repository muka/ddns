FROM scratch

ADD ./build/ddns /ddns

ENTRYPOINT ["/ddns"]
