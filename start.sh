#!/bin/bash

adb start-server
adb devices

streamers=( $ADB_DEVICES )

for i in "${streamers[@]}"
  do
    adb connect $i
  done

files=( prebmitune1.sh bmitune1.sh stopbmitune1.sh prebmitune2.sh bmitune2.sh stopbmitune2.sh prebmitune3.sh bmitune3.sh stopbmitune3.sh prebmitune4.sh bmitune4.sh stopbmitune4.sh )

for i in "${files[@]}"
  do
    if [ ! -f /opt/scripts/$i ]; then
      cp /go/src/github.com/bnhf/$i /opt/scripts \
      && chmod +x /opt/scripts/$i \
      && echo "No existing $i found"
    else
      echo "Existing $i found, and will be used"
  fi
done

/opt/androidhdmi-for-channels$TUNERS
