#!/bin/bash

adb start-server

streamers=( $STREAMER1_IP $STREAMER2_IP $STREAMER3_IP $STREAMER4_IP )

for i in "${streamers[@]}"
  do
    if [ ! -z $i ]; then
      adb connect $i
    fi
  done

adb devices

files=( prebmitune.sh bmitune.sh stopbmitune.sh )

for i in "${files[@]}"
  do
    if [ ! -f /opt/scripts/sample/yttv/$i ]; then
      cp /go/src/github.com/bnhf/sample/yttv/$i /opt/scripts/sample/yttv \
      && chmod +x /opt/scripts/sample/yttv/$i \
      && echo "No existing $i found"
    else
      echo "Existing $i found, and will be used"
  fi
done

/opt/androidhdmi-for-channels$TUNERS
