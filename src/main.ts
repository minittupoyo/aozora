// The video player page. It plays what the native UI asks for and reports how
// the video is doing; all controls are drawn natively over this page.
import Hls from "hls.js";
import { PlayerBridge, events, type PlayerState } from "./mygo";

const video = document.querySelector<HTMLVideoElement>("#video")!;
let hls: Hls | null = null;
let started = false;
let error = "";

function report() {
  const state: PlayerState = {
    started,
    paused: video.paused,
    buffering: video.readyState < 3 && !video.paused && !video.ended,
    time: video.currentTime || 0,
    duration: Number.isFinite(video.duration) ? video.duration : 0,
    volume: video.volume,
    muted: video.muted,
    rate: video.playbackRate,
    pipSupported: document.pictureInPictureEnabled === true,
    pip: document.pictureInPictureElement === video,
    error,
  };
  void PlayerBridge.report(state);
}

// Report at most every 250 ms while time passes, at once for everything else.
let last = 0;
video.addEventListener("timeupdate", () => {
  const now = performance.now();
  if (now - last < 250) return;
  last = now;
  report();
});
for (const ev of [
  "play", "pause", "playing", "waiting", "seeking", "seeked", "canplay", "ended",
  "durationchange", "volumechange", "ratechange", "enterpictureinpicture", "leavepictureinpicture",
]) {
  video.addEventListener(ev, () => {
    if (ev === "playing") started = true;
    report();
  });
}
video.addEventListener("error", () => {
  error = "動画を再生できませんでした";
  report();
});

function stop() {
  hls?.destroy();
  hls = null;
  video.pause();
  video.removeAttribute("src");
  video.load();
  started = false;
  error = "";
  if (document.pictureInPictureElement) document.exitPictureInPicture().catch(() => {});
}

function load(url: string) {
  stop();
  const play = () => video.play().catch(() => {});
  if (Hls.isSupported()) {
    hls = new Hls();
    hls.on(Hls.Events.ERROR, (_, d) => {
      if (!d.fatal) return;
      error = `再生エラー: ${d.details}`;
      report();
    });
    // play() only sticks once the source is attached and the manifest is parsed
    hls.once(Hls.Events.MANIFEST_PARSED, play);
    hls.loadSource(url);
    hls.attachMedia(video);
  } else {
    video.addEventListener("loadedmetadata", play, { once: true });
    video.src = url;
  }
  report();
}

events.playerCmd.on(({ op, url, value }) => {
  switch (op) {
    case "load":
      return load(url ?? "");
    case "stop":
      stop();
      return report();
    case "play":
      return void video.play().catch(() => {});
    case "pause":
      return video.pause();
    case "seek":
      return void (video.currentTime = value ?? 0);
    case "skip":
      if (Number.isFinite(video.duration))
        video.currentTime = Math.min(Math.max(video.currentTime + (value ?? 0), 0), video.duration);
      return;
    case "volume":
      video.volume = value ?? 1;
      video.muted = (value ?? 1) === 0;
      return saveVolume();
    case "mute":
      video.muted = (value ?? 0) === 1;
      if (!video.muted && video.volume === 0) video.volume = 1;
      return saveVolume();
    case "rate":
      return void (video.playbackRate = value ?? 1);
    case "pip":
      if (document.pictureInPictureElement) return void document.exitPictureInPicture().catch(() => {});
      return void video.requestPictureInPicture().catch(() => {});
  }
});

function saveVolume() {
  try {
    localStorage.setItem("tver.volume", JSON.stringify({ v: video.volume, muted: video.muted }));
  } catch {}
}

// restore the last volume
try {
  const saved = JSON.parse(localStorage.getItem("tver.volume") ?? "null");
  if (saved) {
    video.volume = Math.min(1, Math.max(0, Number(saved.v) || 0));
    video.muted = !!saved.muted;
  }
} catch {}
report();
