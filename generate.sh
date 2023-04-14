#!/usr/bin/env zsh

for i in $(shuf -i 1-100);do
    fn=$((100 - i))
    dd bs=1024 count=`shuf -i 1-16 -n 1` if=/dev/zero of="./data/$fn"
    sleep 0.3
done
