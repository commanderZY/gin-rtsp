package service

import (
	"errors"
	"fmt"
	"ginrtsp/serializer"
	"ginrtsp/util"
	"io"
	"os/exec"
	"strings"
	"sync"

	"time"

	uuid "github.com/satori/go.uuid"
)

// RTSPTransSrv RTSP 转换服务 struct
type RTSPTransSrv struct {
	URL string `form:"url" json:"url" binding:"required,min=1"`
}

// processMap FFMPEG 进程刷新通道，未在指定时间刷新的流将会被关闭
var processMap sync.Map

var closeRTSPPlayCh string

// Service RTSP 转换服务
func (service *RTSPTransSrv) Service() *serializer.Response {
	simpleString := strings.Replace(service.URL, "//", "/", 1)
	splitList := strings.Split(simpleString, "/")

	if splitList[0] != "rtsp:" && len(splitList) < 2 {
		return &serializer.Response{
			Code: 400,
			Msg:  "不是有效的 RTSP 地址",
		}
	}

	// 多个客户端需要播放相同的RTSP流地址时，保证返回WebSocket地址相同
	processCh := uuid.NewV3(uuid.NamespaceURL, simpleString).String()
	if ch, ok := processMap.Load(processCh); ok {
		*ch.(*chan struct{}) <- struct{}{}
	} else {
		reflush := make(chan struct{})
		if cmd, stdin, err := runFFMPEG(service.URL, processCh); err != nil {
			return serializer.Err(400, err.Error(), err)
		} else {
			go keepFFMPEG(cmd, stdin, &reflush, processCh)
		}
	}

	playURL := fmt.Sprintf("/stream/live/%s", processCh)
	return serializer.BuildRTSPPlayPathResponse(playURL)
}

func keepFFMPEG(cmd *exec.Cmd, stdin io.WriteCloser, ch *chan struct{}, playCh string) {
	processMap.Store(playCh, ch)
	defer func() {
		processMap.Delete(playCh)
		close(*ch)
		_ = stdin.Close()
		util.Log().Info("Stop translate rtsp id %v", playCh)
	}()

	for {
		select {
		case <-*ch:
			util.Log().Info("Reflush channel %s", playCh)

		case <-time.After(1 * time.Second):
			util.Log().Debug("closeRTSPPlayCh %v", closeRTSPPlayCh)
			if closeRTSPPlayCh == playCh && closeRTSPPlayCh != "" {
				closeRTSPPlayCh = ""
				_, _ = stdin.Write([]byte ( "q" ))
				err := cmd.Wait()
				if err != nil {
					util.Log().Error("Run ffmpeg err %v", err.Error())
				} else {
					util.Log().Debug("Stop this rtsp")
				}
				return
			}
		}
	}
}

func runFFMPEG(rtsp string, playCh string) (*exec.Cmd, io.WriteCloser, error) {
	params := []string{
		"-rtsp_transport",
		"tcp",
		//"-re",
		"-timeout",
		"5000000",
		"-i",
		rtsp,
		"-q",
		"0",
		"-f",
		"mpegts",
		"-fflags",
		"nobuffer",
		"-codec:v",
		"mpeg1video",
		//"-s", "960x540", "-b:v", "1500k", "-r", "30", "-bf", "0",
		"-codec:a",
		"mp2",
		"-ar", "44100", "-ac", "1", "-b:a", "128k",
		"-muxdelay", "0.001",
		fmt.Sprintf("http://127.0.0.1:3000/stream/upload/%s", playCh),
	}
	if strings.Contains(rtsp, "main_stream") {
		params = []string{
			"-rtsp_transport",
			"tcp",
			//"-re",
			"-timeout",
			"5000000",
			"-i",
			rtsp,
			"-q",
			"0",
			"-f",
			"mpegts",
			"-fflags",
			"nobuffer",
			"-codec:v",
			"mpeg1video",
			"-s", "960x540", "-b:v", "1000k", "-r", "20", "-bf", "0",
			//"-b:v", "1000k", "-r", "20", "-bf", "0",
			"-codec:a",
			"mp2",
			"-ar", "44100", "-ac", "1", "-b:a", "128k",
			"-muxdelay", "0.001",
			fmt.Sprintf("http://127.0.0.1:3000/stream/upload/%s", playCh),
			//"out.ts",
		}
	}

	util.Log().Debug("FFmpeg cmd: ./ffmpeg %v", strings.Join(params, " "))
	cmd := exec.Command("./ffmpeg", params...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	stdin, err := cmd.StdinPipe()
	if err != nil {
		util.Log().Error("Get ffmpeg stdin err:%v", err.Error())
		return nil, nil, errors.New("拉流进程启动失败")
	}

	err = cmd.Start()
	if err != nil {
		util.Log().Info("Start ffmpeg err: %v", err.Error())
		return nil, nil, errors.New("打开摄像头视频流失败")
	}
	util.Log().Info("Translate rtsp %v to %v", rtsp, playCh)
	return cmd, stdin, nil
}
