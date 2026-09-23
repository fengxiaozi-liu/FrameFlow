package media

import (
 "context"
 "os"
 "os/exec"
 "path/filepath"
 "testing"

 "github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func TestComposeMixedAspectClipsWithVoiceAndLoopedMusic(t *testing.T) {
 if _,err:=exec.LookPath(executable("FRAMEFLOW_FFMPEG_PATH","ffmpeg"));err!=nil {t.Skip("ffmpeg unavailable")}
 if _,err:=exec.LookPath(executable("FRAMEFLOW_FFPROBE_PATH","ffprobe"));err!=nil {t.Skip("ffprobe unavailable")}
 dir:=t.TempDir()
 clip1:=filepath.Join(dir,"clip1.mp4")
 clip2:=filepath.Join(dir,"clip2.mp4")
 voice:=filepath.Join(dir,"voice.wav")
 music:=filepath.Join(dir,"music.wav")
 if err:=runFFmpeg(context.Background(),"-y","-f","lavfi","-i","testsrc2=size=320x240:rate=30","-f","lavfi","-i","sine=frequency=440:sample_rate=48000","-t","1","-c:v","libx264","-pix_fmt","yuv420p","-c:a","aac",clip1);err!=nil {t.Fatal(err)}
 if err:=runFFmpeg(context.Background(),"-y","-f","lavfi","-i","testsrc2=size=240x320:rate=30","-t","1","-c:v","libx264","-pix_fmt","yuv420p",clip2);err!=nil {t.Fatal(err)}
 if err:=runFFmpeg(context.Background(),"-y","-f","lavfi","-i","sine=frequency=660:sample_rate=48000","-t","0.7",voice);err!=nil {t.Fatal(err)}
 if err:=runFFmpeg(context.Background(),"-y","-f","lavfi","-i","sine=frequency=220:sample_rate=48000","-t","0.5",music);err!=nil {t.Fatal(err)}
 a,err:=Probe(context.Background(),clip1);if err!=nil {t.Fatal(err)}
 b,err:=Probe(context.Background(),clip2);if err!=nil {t.Fatal(err)}
 composer:=Composer{UploadDir:dir,WorkDir:filepath.Join(dir,"work")}
 saved,duration,err:=composer.Compose(context.Background(),"test",task.CompositionInput{
  Clips:[]task.CompositionClip{
   {SceneID:"a",VersionID:"v1",MediaPath:"/media/clip1.mp4",DurationSeconds:a.Duration,VoicePath:"/media/voice.wav"},
   {SceneID:"b",VersionID:"v2",MediaPath:"/media/clip2.mp4",DurationSeconds:b.Duration},
  },MusicPath:"/media/music.wav",MusicVolume:0.25,SourceVolume:0.8,VoiceVolume:1,
  AspectRatio:"9:16",Resolution:"720P",
 },func(int,task.Stage){})
 if err!=nil {t.Fatal(err)}
 if saved!="/media/composition-test.mp4" || duration<1.7 || duration>2.4 {t.Fatal(saved,duration)}
 result,err:=Probe(context.Background(),filepath.Join(dir,"composition-test.mp4"))
 if err!=nil || result.Width!=720 || result.Height!=1280 || !result.HasAudio {t.Fatal(result,err)}
 if _,err:=os.Stat(filepath.Join(dir,"work"));err!=nil {t.Fatal("work directory missing",err)}
 entries,err:=os.ReadDir(filepath.Join(dir,"work"));if err!=nil || len(entries)!=0 {t.Fatal("temporary files not cleaned",entries,err)}
}
