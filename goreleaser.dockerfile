FROM scratch
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/motif /usr/bin/motif
ENTRYPOINT ["/usr/bin/motif"]
