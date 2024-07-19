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

start_time=0900
end_time=1730

while true; do
    current_time=$(TZ='Asia/Seoul' date +"%H%M")

    if [ "$current_time" -ge "$start_time" ] && [ "$current_time" -le "$end_time" ]; then
        if ! pgrep -x "video-viewer" > /dev/null; then
            echo "녹화 시작 시간입니다. video-viewer를 실행합니다."
            video-viewer csi://"$VIDEO_INPUT" "$SERVER_RTP_SET" --input-width=1280 --input-height=720 --input-rate=30/1 --input-codec=mjpeg --output-codec=h264 --output-encoder=cpu --bitrate=2000000 --headless > /var/log/video-viewer.log 2>&1 &
        fi
    else
        if pgrep -x "video-viewer" > /dev/null; then
            echo "녹화 시간이 아닙니다. video-viewer를 종료합니다."
            pkill -2 -x "video-viewer"
        fi
    fi

    # 로그 파일 모니터링을 백그라운드에서 실행
    if pgrep -x "video-viewer" > /dev/null; then
        (tail -f /var/log/video-viewer.log | while read line; do
            echo "$line"
            if [[ "$line" == *"gstCamera::Capture() -- a timeout occurred waiting for the next image buffer"* ]]; then
                echo "Timeout detected. Restarting video-viewer..."
                pkill -2 -x "video-viewer"
                service nvargus-daemon restart
                break
            elif [[ "$line" == *"video-viewer:  failed to create output stream"* ]]; then
                echo "다른 조건문 matched. Restarting video-viewer..."
                pkill -2 -x "video-viewer"
                service nvargus-daemon restart
                break
            fi
        done) &
    fi

    # 잠시 대기 (너무 빠른 재시작 방지)
    sleep 60
done