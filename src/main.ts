import "./ui";
import "@fontsource-variable/google-sans";
import "@fontsource-variable/noto-sans-jp";
import { snackbar, type Chip, type TextField } from "./ui";
import { openPlayer, playerFull } from "./player";
import { Tver, type Genre, type Item, type Section, type YtdlpInfo } from "./mygo";

const $ = <T extends Element = HTMLElement>(s: string) => document.querySelector<T>(s)!;
const view = $("#view");
const input = $<TextField>("#q");

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className?: string, text?: string) {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text !== undefined) e.textContent = text;
  return e;
}

function card(it: Item): HTMLElement {
  const b = el("mdui-card", "card");
    const media = el("div", "media");
  if (it.thumbnail) {
    const img = el("img");
    img.loading = "lazy";
    img.alt = "";
    img.src = it.thumbnail;
    img.onerror = () => img.remove();
    media.append(img);
  }
  if (it.rank) media.append(el("span", "rank", String(it.rank)));
  else if (it.type === "series") media.append(el("span", "badge", "シリーズ"));
  const text = el("div", "text");
  text.append(el("div", "t", it.title), el("div", "s", it.subtitle));
  b.append(media, text);
  b.setAttribute("clickable", "");
  b.onclick = () => (it.type === "series" ? openSeries(it) : openPlayer(it.id, it.title));
  return b;
}

const EMPTY_ICON =
  '<svg viewBox="0 0 24 24"><path d="M21 3H3c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h5v2h8v-2h5c1.1 0 1.99-.9 1.99-2L23 5c0-1.1-.9-2-2-2m0 14H3V5h18z"/></svg>';

function empty(text: string) {
  const d = el("div", "empty");
  d.innerHTML = EMPTY_ICON;
  d.append(el("div", undefined, text));
  view.replaceChildren(d);
}

function sections(secs: Section[]) {
  view.replaceChildren(
    ...secs.flatMap((s) => {
      const row = el("div", "row");
      row.append(...s.items.map(card));
      return [el("h2", "section-title", s.title), row];
    }),
  );
}

// A single-choice filter chip; the caller marks the chosen one with `selected`.
function chip(label: string, onpick: () => void): Chip {
  const c = el("mdui-chip", undefined, label);
  c.setAttribute("variant", "filter");
  c.addEventListener("click", onpick);
  return c;
}

function chipSet(...chips: HTMLElement[]) {
  const set = el("div", "chips");
  set.append(...chips);
  return set;
}

// Views the user can go back to: the nodes (with their handlers) and scroll offset.
const history: { nodes: Node[]; scroll: number }[] = [];

const BACK_ICON =
  '<svg viewBox="0 0 24 24"><path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20z"/></svg>';

function pushView() {
  history.push({ nodes: [...view.childNodes], scroll: scrollY });
}

function goBack() {
  const prev = history.pop();
  if (!prev) return false;
  view.replaceChildren(...prev.nodes);
  scrollTo({ top: prev.scroll });
  return true;
}

function grid(title: string, items: Item[]) {
  const g = el("div", "grid");
  g.append(...items.map(card));
  const head = el("div", "page-header");
  if (history.length) {
    const back = el("mdui-button-icon");
    back.ariaLabel = "戻る";
    back.innerHTML = BACK_ICON;
    back.onclick = goBack;
    head.append(back);
  }
  head.append(el("h2", "section-title", title));
  view.replaceChildren(head, g);
  scrollTo({ top: 0 });
}

function snack(text: string) {
  snackbar({ message: text, autoCloseDelay: 6000, closeable: true });
}

async function load(fn: () => Promise<void>) {
  $("#progress").hidden = false;
  try {
    await fn();
  } catch (e) {
    snack(`エラー: ${e}`);
    if (!view.children.length) empty(`読み込めませんでした\n${e}`);
  } finally {
    $("#progress").hidden = true;
  }
}

// The home page is reused by the category browser, so fetch it once per visit to Home.
let homeData: Promise<Section[]> | null = null;
const getHome = (fresh = false) => {
  if (fresh || !homeData) {
    homeData = Tver.home();
    homeData.catch(() => (homeData = null));
  }
  return homeData;
};

const showHome = () =>
  load(async () => {
    const secs = await getHome(true);
    secs.length ? sections(secs) : empty("コンテンツがありません");
  });

const CATEGORIES = [
  ["drama", "ドラマ"],
  ["variety", "バラエティ"],
  ["anime", "アニメ"],
  ["news", "報道・ニュース"],
  ["sports", "スポーツ"],
  ["others", "その他"],
] as const;

// Category -> the genre of TVer's episode ranking that matches it.
const RANK_GENRE: Record<string, string> = {
  drama: "drama",
  variety: "variety",
  anime: "anime",
  news: "news_documentary",
  sports: "sports",
  others: "other",
};

const episodeRankings = new Map<string, Promise<Item[]>>();
function episodeRanking(genre: string) {
  let p = episodeRankings.get(genre);
  if (!p) {
    p = Tver.ranking("episode", genre);
    p.catch(() => episodeRankings.delete(genre));
    episodeRankings.set(genre, p);
  }
  return p;
}

// Rankings of programs or of episodes, by genre, up to 50 places.
let rankKind: "series" | "episode" = "series";
let rankGenre = "all";

const showRanking = () => {
  setNav("ranking");
  history.length = 0;
  input.value = "";
  return load(async () => {
    const genres: Genre[] = await Tver.rankingGenres(rankKind);
    if (!genres.length) return empty("ランキングを取得できませんでした");
    if (!genres.some((g) => g.id === rankGenre)) rankGenre = genres[0]!.id;

    const kinds = chipSet(
      ...([["series", "番組ランキング"], ["episode", "エピソードランキング"]] as const).map(([kind, label]) => {
        const c = chip(label, () => {
          rankKind = kind;
          showRanking();
        });
        c.selected = kind === rankKind;
        return c;
      }),
    );

    const body = el("div", "ranking-body");
    const buttons = new Map<string, Chip>();
    let seq = 0;
    const render = async (id: string) => {
      const mine = ++seq;
      rankGenre = id;
      buttons.forEach((b, k) => (b.selected = k === id));
      const items = await Tver.ranking(rankKind, id);
      if (mine !== seq) return; // a newer genre was picked meanwhile
      const g = el("div", "grid");
      g.append(...items.map(card));
      body.replaceChildren(items.length ? g : el("p", "muted", "ランキングがありません"));
    };
    for (const g of genres) buttons.set(g.id, chip(g.title, () => load(() => render(g.id))));
    const chips = chipSet(...buttons.values());
    view.replaceChildren(el("h2", "section-title", "ランキング"), kinds, chips, body);
    scrollTo({ top: 0 });
    await render(rankGenre);
  });
};

// Browse by genre: one chip per category, each showing that genre's rows from the home page.
const showCategories = () => {
  setNav("categories");
  history.length = 0;
  input.value = "";
  return load(async () => {
    const all = await getHome();
    const groups = CATEGORIES.map(([id, label]) => ({
      id,
      label,
      // the home page only carries a top-10 ranking; the full one is added below
      secs: all.filter((s) => s.category === id && !s.title.includes("ランキング")),
    })).filter((g) => g.secs.length);
    if (!groups.length) return empty("カテゴリを取得できませんでした");

    const body = el("div", "category-body");
    const buttons: Chip[] = [];
    let shown = -1;
    const select = (i: number) => {
      shown = i;
      buttons.forEach((b, k) => (b.selected = k === i));
      const title = el("h2", "section-title", "ランキング");
      const rankRow = el("div", "row");
      title.hidden = rankRow.hidden = true;
      body.replaceChildren(
        title,
        rankRow,
        ...groups[i]!.secs.flatMap((s) => {
          const row = el("div", "row");
          row.append(...s.items.map(card));
          return [el("h2", "section-title", s.title), row];
        }),
      );
      scrollTo({ top: 0 });
      episodeRanking(RANK_GENRE[groups[i]!.id] ?? "all")
        .then((items) => {
          if (shown !== i || !items.length) return;
          rankRow.append(...items.map(card));
          title.hidden = rankRow.hidden = false;
        })
        .catch(() => {});
    };
    groups.forEach((g, i) => buttons.push(chip(`${g.label}  ${g.secs.length}`, () => select(i))));
    const chips = chipSet(...buttons);
    select(0);
    view.replaceChildren(el("h2", "section-title", "カテゴリ"), chips, body);
  });
};

// A series page: one chip per season (本編, メイキング, ...) filtering the episode grids.
function seasons(title: string, secs: Section[], startWithAll = false) {
  const head = el("div", "page-header");
  if (history.length) {
    const back = el("mdui-button-icon");
    back.ariaLabel = "戻る";
    back.innerHTML = BACK_ICON;
    back.onclick = goBack;
    head.append(back);
  }
  head.append(el("h2", "section-title", title));

  const body = el("div", "season-body");
  const ALL = -1;
  const buttons: Chip[] = [];
  const select = (i: number) => {
    buttons.forEach((b, k) => (b.selected = k === i + 1));
    body.replaceChildren(
      ...secs.flatMap((s, k) => {
        if (i !== ALL && k !== i) return [];
        const g = el("div", "grid");
        g.append(...s.items.map(card));
        return [el("h3", "season-title", `${s.title}  ${s.items.length}`), g];
      }),
    );
  };
  const entries: [string, number][] = [["すべて", ALL], ...secs.map((s, k): [string, number] => [s.title, k])];
  for (const [label, i] of entries) buttons.push(chip(label, () => select(i)));
  const chips = chipSet(...buttons);
  select(startWithAll ? ALL : 0);
  view.replaceChildren(head, chips, body);
  scrollTo({ top: 0 });
}

const openSeries = (it: Item) =>
  load(async () => {
    const secs = await Tver.seriesEpisodes(it.id);
    if (!secs.length) return snack("エピソードがありません");
    pushView();
    seasons(it.title, secs);
  });

const SOURCE_LABEL = { managed: "アプリ内のコピーを使用中", path: "PATH 上の yt-dlp を使用中", none: "見つかりません" } as const;

function settingsRow(label: string, value: string) {
  const row = el("div", "settings-row");
  row.append(el("div", "settings-label", label), el("div", "settings-value", value));
  return [document.createElement("mdui-divider"), row];
}

function showSettings() {
  setNav("settings");
  history.length = 0;
  input.value = "";

  const card = el("section", "settings-card");
  const body = el("div", "settings-body");
  const actions = el("div", "settings-actions");
  const bar = el("mdui-linear-progress");
  bar.hidden = true;
  card.append(
    el("h2", "settings-title", "yt-dlp"),
    el("p", "settings-desc", "再生URLの取得に使います。アプリ専用のコピーをダウンロードすると、PATH にあるものより優先して使います。"),
    body,
    bar,
    actions,
  );
  view.replaceChildren(el("h2", "section-title", "設定"), card);
  scrollTo({ top: 0 });

  let busy = false;
  const render = (info: YtdlpInfo) => {
    body.replaceChildren(
      ...settingsRow("状態", SOURCE_LABEL[info.source as keyof typeof SOURCE_LABEL] ?? info.source),
      ...settingsRow("バージョン", info.version || "—"),
      ...settingsRow("場所", info.path || "—"),
    );
    const install = el("mdui-button", undefined, info.managed ? "最新版に更新" : "ダウンロード");
    install.disabled = busy;
    install.onclick = () => run(() => Tver.ytdlpInstall(), install.textContent!);
    actions.replaceChildren(install);
    if (info.managed) {
      const remove = el("mdui-button", undefined, "アプリ内のコピーを削除");
      remove.setAttribute("variant", "text");
      remove.disabled = busy;
      remove.onclick = () => run(() => Tver.ytdlpRemove(), "削除");
      actions.append(remove);
    }
  };
  let last: YtdlpInfo | null = null;
  const run = async (fn: () => Promise<YtdlpInfo>, label: string) => {
    busy = true;
    bar.hidden = false;
    if (last) render(last);
    try {
      last = await fn();
      snack(`${label}が完了しました`);
    } catch (e) {
      snack(`エラー: ${e}`);
    } finally {
      busy = false;
      bar.hidden = true;
      if (last) render(last);
    }
  };
  body.append(el("div", "settings-value", "確認中…"));
  Tver.ytdlpStatus().then((info) => {
    last = info;
    render(info);
  });
}

const NAV = [
  ["home", "ホーム", "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"],
  ["categories", "カテゴリ", "M4 8h4V4H4zm6 12h4v-4h-4zm-6 0h4v-4H4zm0-6h4v-4H4zm6 0h4v-4h-4zm6-10v4h4V4zm-6 4h4V4h-4zm6 6h4v-4h-4zm0 6h4v-4h-4z"],
  ["ranking", "ランキング", "M7.5 21H2V9h5.5zm7.25-18h-5.5v18h5.5zM22 11h-5.5v10H22z"],
  ["search", "検索", "M15.5 14h-.79l-.28-.27A6.47 6.47 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14"],
  ["settings", "設定", "M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 0 0 .12-.61l-1.92-3.32a.488.488 0 0 0-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.484.484 0 0 0-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58a.49.49 0 0 0-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6"],
] as const;
type Page = (typeof NAV)[number][0];

const OPEN: Record<Page, () => unknown> = {
  home: () => goHome(),
  categories: () => showCategories(),
  ranking: () => showRanking(),
  search: () => input.focus(),
  settings: () => showSettings(),
};

// The rail (wide windows) and the bar (compact windows) carry the same items; CSS shows one.
const navs = [$<HTMLElement & { value: string }>("#rail"), $<HTMLElement & { value: string }>("#bar")];
for (const nav of navs) {
  for (const [page, label, path] of NAV) {
    const item = el(nav.localName + "-item" as "div", undefined, label);
    item.setAttribute("value", page);
    item.insertAdjacentHTML("afterbegin", `<svg slot="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="${path}" /></svg>`);
    item.onclick = () => OPEN[page]();
    nav.append(item);
  }
}

function setNav(page: Page) {
  for (const nav of navs) nav.value = page;
}

function goHome() {
  input.value = "";
  setNav("home");
  history.length = 0;
  showHome();
}

$("#home").onclick = goHome;
input.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.isComposing) search();
});
function search() {
  const q = input.value.trim();
  if (!q) return goHome();
  setNav("search");
  history.length = 0;
  load(async () => {
    const secs = await Tver.search(q);
    secs.length ? seasons(`「${q}」の検索結果`, secs, true) : empty("見つかりませんでした");
  });
}

// back: Alt+←, the mouse back button, or Esc (the player handles its own Esc)
addEventListener("keydown", (e) => {
  if (playerFull() || e.defaultPrevented) return;
  const typing = e.target === input;
  if ((e.key === "ArrowLeft" && e.altKey) || (e.key === "Escape" && !typing)) {
    if (goBack()) e.preventDefault();
  }
});
addEventListener("mouseup", (e) => {
  if (e.button === 3 && !playerFull() && goBack()) e.preventDefault();
});

showHome();
