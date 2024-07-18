#!/bin/bash

# 해당 쉘스크립트 사용 방법 - supervisor, jetson 기준, jetson-inference 설치
# 
# /etc/supervisor/conf.d/supervisord.conf

# [program:recording]
# command=/bin/bash -c "/usr/local/bin/record.sh"
# environment=SERVER_RTP_SET="rtp://192.168.20.22:15000"
# user=root
# autostart=true
# autorestart=true
# stopasgroup=true
# stderr_logfile=/var/log/recording.err.log
# stdout_logfile=/var/log/recording.out.log

cleanup() {
    echo "Caught signal, stopping..."
    # SIGINT 시그널으로 gst-launch-1.0 프로세스 종료
    pkill -2 -P $$
    exit
}

# SIGINT와 SIGTERM을 cleanup 함수로 라우팅
# trap 명령어를 통해 supervisor가 종료 명령 (기본값 SIGTERM) 시그널을 보냈을 때 해당 트랩의 함수가 작동
trap cleanup INT TERM

VIDEO_INPUT=$(ls /dev/video* | grep -o "[0-9]")

while true; do
    # 현재 시간을 가져오기 (HHMM 형식, (KST, UTC+9) 기준)
    current_time=$(TZ='Asia/Seoul' date +"%H%M")

    # 아침 8:30 (0830)과 저녁 5:30 (1730) 시간 정의
    start_time=0900
    end_time=1730

    # 현재 시간이 실행 시간 내에 있는지 확인
    if [ "$current_time" -ge "$start_time" ] && [ "$current_time" -le "$end_time" ]; then
        # 명령어 실행
        video-viewer csi://"$VIDEO_INPUT" "$SERVER_RTP_SET" --input-width=1280 --input-height=720 --input-rate=30/1 --input-codec=mjpeg --output-codec=h264 --output-encoder=cpu --bitrate=2000000 --headless > /var/log/video-viewer.log 2>&1 &
    else
        echo "현재 시간은 명령어 실행 시간이 아닙니다."
        sleep 60
        continue
    fi

    # 로그를 지속적으로 확인
    tail -f /var/log/video-viewer.log | while read line; do
        echo "$line"
        if [[ "$line" == *"gstCamera::Capture() -- a timeout occurred waiting for the next image buffer"* ]]; then
            echo "Timeout detected. Restarting gst-launch-1.0..."
            pkill -2 -P $$
            break
        elif [[ "$line" == *"video-viewer:  failed to create output stream"* ]]; then
            echo "다른 조건문 matched. Restarting gst-launch-1.0..."
            pkill -2 -P $$
            break
        fi
    done
    # 잠시 대기 (너무 빠른 재시작 방지)
    sleep 5
    service nvargus-daemon restart
    sleep 5
done