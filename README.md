# Merry-Go Project
Merry-Go-Around
SnackGo는 임시 이름
원형 큐 형식의 순환하는 비디오 업로드 서버 구현, 회전목마처럼 계속해서 돌아가는 HLS 기반의 영상 스트리밍 서버를 구현하는게 목표
사용자가 업로드를 통하여 Merry-Go 스트리밍을 만들 수 있음


## TODO ---

### BackEnd
- Merry-Go에서 영상 빠져나오는 로직 추가

### FrontEnd
- 유저가 보낸 메세지 영상 위로 니코동/티비플 처럼 날아가게 하기
- 업로드 성공 시 알림이나 체크 가능하도록 변경
- 현재 영상 큐 갯수 보여주기 ex) 10/10 -> 꽉찬 상태 - 업로드 불가 / 8/10 2개 빈 상태 - 업로드 가능


# HLS 관련 - playlist.m3u8 내의 설정
- #EXT-X-TARGETDURATION : N 일때, N/2 정도가 playlist.m3u8을 재요청하는 주기가 된다. 깜박임없이 자연스럽게 요청하고 갱신됨
