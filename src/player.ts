import Hls from "hls.js";
import type { Slider } from "./ui";
import { Tver } from "./mygo";

const $ = <T extends HTMLElement>(s: string) => document.querySelector<T>(s)!;

const ICON = {
  play: "M8 5v14l11-7z",
  pause: "M6 19h4V5H6zm8-14v14h4V5z",
  volume:
    "M3 9v6h4l5 5V4L7 9zm13.5 3A4.5 4.5 0 0 0 14 7.97v8.05c1.48-.73 2.5-2.25 2.5-4.02M14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77",
  muted:
    "M16.5 12A4.5 4.5 0 0 0 14 7.97v2.21l2.45 2.45c.03-.2.05-.41.05-.63m2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51A8.8 8.8 0 0 0 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71M4.27 3 3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06a9 9 0 0 0 3.69-1.81L19.73 21 21 19.73l-9-9zM12 4 9.91 6.09 12 8.18z",
  fullscreen: "M7 14H5v5h5v-2H7zm-2-4h2V7h3V5H5zm12 7h-3v2h5v-5h-2zM14 5v2h3v3h2V5z",
  fullscreenExit: "M5 16h3v3h2v-5H5zm3-8H5v2h5V5H8zm6 11h2v-3h3v-2h-5zm2-11V5h-2v5h5V8z",
  back: "M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20z",
  close: "M19 6.41 17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z",
  minimize: "M22 3.41 16.71 8.7 20 12h-8V4l3.29 3.29L20.59 2zM3.41 22l5.29-5.29L12 20v-8H4l3.29 3.29L2 20.59z",
  expand: "M21 11V3h-8l3.29 3.29-10 10L3 13v8h8l-3.29-3.29 10-10z",
};

const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2];
const SEEK_STEP = 10;
const IDLE_MS = 3000;

const root = $("#player");
const video = $<HTMLVideoElement>("#video");
const seek = $<Slider>("#seek");
const volume = $<Slider>("#volume");
const spinner = $("#spinner");
const steps = $("#steps");
const status = $("#pstatus");
const statusText = $("#pstatus-text");
const timeLabel = $("#ptime");
const speedBtn = $("#speed");

let hls: Hls | null = null;
let current: { id: string; title: string } | null = null;
let idleTimer = 0;
let scrubbing = false;

function setIcon(btn: HTMLElement, path: string) {
  btn.querySelector("path")!.setAttribute("d", path);
}

function fmt(sec: number) {
  if (!Number.isFinite(sec) || sec < 0) sec = 0;
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  const mm = h ? String(m).padStart(2, "0") : String(m);
  return `${h ? h + ":" : ""}${mm}:${String(s).padStart(2, "0")}`;
}

function showStatus(text: string, retry = false) {
  spinner.hidden = true;
  statusText.textContent = text;
  $("#retry").hidden = !retry;
  status.hidden = false;
}

function wake() {
  root.classList.remove("idle");
  clearTimeout(idleTimer);
  if (!video.paused) idleTimer = window.setTimeout(() => root.classList.add("idle"), IDLE_MS);
}

function toggle() {
  if (video.paused) video.play().catch(() => {});
  else video.pause();
}

function skip(delta: number) {
  if (!Number.isFinite(video.duration)) return;
  video.currentTime = Math.min(Math.max(video.currentTime + delta, 0), video.duration);
  wake();
}

function setVolume(v: number, muted = v === 0) {
  video.volume = v;
  video.muted = muted;
  try {
    localStorage.setItem("tver.volume", JSON.stringify({ v, muted }));
  } catch {}
}

function syncVolume() {
  const level = video.muted ? 0 : video.volume;
  volume.value = level;
  setIcon($("#mute"), level === 0 ? ICON.muted : ICON.volume);
}

function syncTime() {
  const d = video.duration;
  if (!scrubbing && Number.isFinite(d) && d > 0) {
    seek.value = Math.round((video.currentTime / d) * 1000);
  }
  timeLabel.textContent = `${fmt(video.currentTime)} / ${Number.isFinite(d) ? fmt(d) : "--:--"}`;
}

function syncPlay() {
  const paused = video.paused;
  root.classList.toggle("paused", paused);
  for (const b of [$("#play"), $("#center")]) setIcon(b, paused ? ICON.play : ICON.pause);
  $("#play").setAttribute("aria-label", paused ? "再生" : "一時停止");
  wake();
}

// Full screen is the app window itself (over the taskbar), not the page's own fullscreen.
let nativeFull = false;

function syncFullscreen() {
  setIcon($("#fullscreen"), nativeFull ? ICON.fullscreenExit : ICON.fullscreen);
  $("#fullscreen").setAttribute("aria-label", nativeFull ? "全画面を終了" : "全画面");
}

async function setNativeFull(on: boolean) {
  try {
    nativeFull = await Tver.setFullScreen(on);
  } catch {
    return;
  }
  syncFullscreen();
}

function toggleFullscreen() {
  void setNativeFull(!nativeFull);
}

const pipBtn = $("#pip");
pipBtn.hidden = !document.pictureInPictureEnabled;

function togglePip() {
  if (!document.pictureInPictureEnabled) return;
  if (document.pictureInPictureElement) document.exitPictureInPicture().catch(() => {});
  else video.requestPictureInPicture().catch(() => {});
}

function syncPip() {
  const on = document.pictureInPictureElement === video;
  root.classList.toggle("pip", on);
  $("#pip-note").hidden = !on;
  pipBtn.setAttribute("aria-pressed", String(on));
}

function autoplay() {
  video.play().catch(() => {
    // blocked by the autoplay policy: show the center button instead
    spinner.hidden = true;
    steps.hidden = true;
    root.classList.remove("busy");
    syncPlay();
  });
}

// One line of the preparation list shown before playback starts.
type Step = { ok: (detail?: string) => void; fail: (detail?: string) => void };

function step(label: string): Step {
  const li = document.createElement("li");
  li.dataset.state = "run";
  const mark = document.createElement("span");
  mark.className = "mark";
  const text = document.createElement("span");
  const name = document.createElement("span");
  name.className = "label";
  name.textContent = label;
  const detail = document.createElement("span");
  detail.className = "detail";
  detail.hidden = true;
  text.append(name, detail);
  li.append(mark, text);
  steps.append(li);
  steps.hidden = false;
  const done = (state: string) => (d?: string) => {
    li.dataset.state = state;
    if (d) {
      detail.textContent = d;
      detail.hidden = false;
    }
  };
  return { ok: done("ok"), fail: done("fail") };
}

const firstLine = (e: unknown) => String(e).replace(/^CallError:\s*/, "").split("\n")[0]!;

let playStep: Step | null = null;

async function start() {
  if (!current) return;
  const id = current.id;
  status.hidden = true;
  spinner.hidden = false;
  steps.replaceChildren();
  steps.hidden = true;
  playStep = null;
  root.classList.add("busy");
  try {
    // 1. resolve the HLS URL: yt-dlp first (Brightcove often fails), then Brightcove
    let s;
    const ytdlp = step("yt-dlp で再生URLを取得");
    try {
      s = await Tver.streamYtdlp(id);
      ytdlp.ok();
    } catch (e) {
      ytdlp.fail(firstLine(e));
      const direct = step("Brightcove から再生URLを取得");
      try {
        s = await Tver.streamDirect(id);
        direct.ok();
      } catch (e2) {
        direct.fail(firstLine(e2));
        throw new Error("再生URLを取得できませんでした");
      }
    }
    if (current?.id !== id) return; // closed or switched meanwhile

    // 2. loopback proxy that adds TVer's Origin
    const prep = step("ストリームを準備");
    const url = await Tver.proxy(s.url);
    prep.ok();

    // 3. manifest, then playback
    const manifest = step("マニフェストを読み込み");
    hls?.destroy();
    hls = null;
    const onParsed = () => {
      manifest.ok();
      playStep = step("再生を開始");
      autoplay();
    };
    if (Hls.isSupported()) {
      hls = new Hls();
      hls.on(Hls.Events.ERROR, (_, d) => {
        if (!d.fatal) return;
        manifest.fail(d.details);
        playStep?.fail();
        showStatus(`再生エラー: ${d.details}`, true);
      });
      // play() only sticks once the source is attached and the manifest is parsed
      hls.once(Hls.Events.MANIFEST_PARSED, onParsed);
      hls.loadSource(url);
      hls.attachMedia(video);
    } else {
      video.addEventListener("loadedmetadata", onParsed, { once: true });
      video.src = url;
    }
  } catch (e) {
    showStatus(firstLine(e), true);
  }
}

export const playerOpen = () => !root.hidden;
// the full-size player owns the keyboard; the mini player leaves the page usable
export const playerFull = () => !root.hidden && !root.classList.contains("mini");

const miniBtn = $("#mini");

function setMini(on: boolean) {
  if (on && nativeFull) void setNativeFull(false);
  root.classList.toggle("mini", on);
  setIcon($("#close"), on ? ICON.close : ICON.back);
  $("#close").setAttribute("aria-label", on ? "閉じる" : "戻る");
  setIcon(miniBtn, on ? ICON.expand : ICON.minimize);
  miniBtn.setAttribute("aria-label", on ? "元のサイズに戻す" : "ミニプレイヤー");
  wake();
}

export function openPlayer(id: string, title: string) {
  setMini(false);
  current = { id, title };
  $("#ptitle").textContent = title;
  root.hidden = false;
  root.classList.add("paused");
  seek.value = 0;
  syncTime();
  wake();
  start();
}

export function closePlayer() {
  if (nativeFull) void setNativeFull(false);
  if (document.pictureInPictureElement) document.exitPictureInPicture().catch(() => {});
  clearTimeout(idleTimer);
  hls?.destroy();
  hls = null;
  video.pause();
  video.removeAttribute("src");
  video.load();
  current = null;
  root.classList.remove("busy");
  setMini(false);
  root.hidden = true;
}

// controls
$("#close").onclick = closePlayer;
$("#retry").onclick = start;
$("#play").onclick = toggle;
$("#center").onclick = toggle;
$("#back10").onclick = () => skip(-SEEK_STEP);
$("#fwd10").onclick = () => skip(SEEK_STEP);
$("#fullscreen").onclick = toggleFullscreen;
pipBtn.onclick = togglePip;
miniBtn.onclick = () => setMini(!root.classList.contains("mini"));
video.addEventListener("enterpictureinpicture", syncPip);
video.addEventListener("leavepictureinpicture", syncPip);
$("#mute").onclick = () => {
  const level = video.muted ? 0 : video.volume;
  if (level === 0) setVolume(video.volume || 1, false);
  else setVolume(video.volume, true);
};
speedBtn.onclick = () => {
  const i = SPEEDS.indexOf(video.playbackRate);
  video.playbackRate = SPEEDS[(i + 1) % SPEEDS.length]!;
};
volume.oninput = () => setVolume(volume.value ?? 0);
seek.addEventListener("pointerdown", () => (scrubbing = true));
seek.addEventListener("pointerup", () => (scrubbing = false));
seek.addEventListener("change", () => (scrubbing = false));
seek.oninput = () => {
  if (Number.isFinite(video.duration)) video.currentTime = ((seek.value ?? 0) / 1000) * video.duration;
};

// surface: click toggles, double click toggles fullscreen
let clickTimer = 0;
video.onclick = () => {
  if (root.classList.contains("mini")) return setMini(false);
  clearTimeout(clickTimer);
  clickTimer = window.setTimeout(toggle, 220);
};
video.ondblclick = () => {
  if (root.classList.contains("mini")) return;
  clearTimeout(clickTimer);
  toggleFullscreen();
};

root.addEventListener("mousemove", wake);
root.addEventListener("pointerdown", wake);

// media events
video.addEventListener("play", syncPlay);
video.addEventListener("pause", syncPlay);
video.addEventListener("timeupdate", syncTime);
video.addEventListener("durationchange", syncTime);
video.addEventListener("volumechange", syncVolume);
video.addEventListener("ratechange", () => (speedBtn.textContent = `${video.playbackRate}x`));
video.addEventListener("waiting", () => (spinner.hidden = false));
video.addEventListener("seeking", () => (spinner.hidden = false));
for (const ev of ["playing", "canplay", "seeked"]) video.addEventListener(ev, () => (spinner.hidden = true));
video.addEventListener("playing", () => {
  status.hidden = true;
  playStep?.ok();
  playStep = null;
  steps.hidden = true;
  root.classList.remove("busy");
});
video.addEventListener("error", () => showStatus("動画を再生できませんでした", true));

// keyboard
addEventListener("keydown", (e) => {
  if (!playerFull() || e.ctrlKey || e.metaKey || e.altKey) return;
  const handled = (fn: () => void) => {
    e.preventDefault();
    fn();
    wake();
  };
  switch (e.key) {
    case "Escape":
      return handled(() => (nativeFull ? void setNativeFull(false) : closePlayer()));
    case " ":
    case "k":
      if (e.target instanceof HTMLElement && e.target.closest("button, mdui-button, mdui-button-icon")) return;
      return handled(toggle);
    case "ArrowLeft":
    case "j":
      return handled(() => skip(-SEEK_STEP));
    case "ArrowRight":
    case "l":
      return handled(() => skip(SEEK_STEP));
    case "ArrowUp":
      return handled(() => setVolume(Math.min(1, (video.muted ? 0 : video.volume) + 0.1), false));
    case "ArrowDown":
      return handled(() => setVolume(Math.max(0, (video.muted ? 0 : video.volume) - 0.1)));
    case "m":
      return handled(() => $("#mute").click());
    case "f":
      return handled(toggleFullscreen);
    case "p":
      return handled(togglePip);
    case "i":
      return handled(() => setMini(true));
  }
});

// restore the last volume
try {
  const saved = JSON.parse(localStorage.getItem("tver.volume") ?? "null");
  if (saved) {
    video.volume = Math.min(1, Math.max(0, Number(saved.v) || 0));
    video.muted = !!saved.muted;
  }
} catch {}
syncVolume();
