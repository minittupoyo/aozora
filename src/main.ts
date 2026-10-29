import "@fontsource-variable/google-sans";
import "@fontsource-variable/noto-sans-jp";
import { openPlayer, playerFull } from "./player";
import { Tver, type Genre, type Item, type Section, type YtdlpInfo } from "./mygo";

const $ = <T extends HTMLElement>(s: string) => document.querySelector<T>(s)!;
const view = $("#view");
const input = $<HTMLInputElement>("#q");
const clearBtn = $("#clear");
let snackTimer = 0;

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className?: string, text?: string) {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text !== undefined) e.textContent = text;
  return e;
}

function card(it: Item): HTMLElement {
  const b = el("button", "card");
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
    const back = el("button", "icon-button");
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
  const s = $("#snackbar");
  s.textContent = text;
  s.hidden = false;
  clearTimeout(snackTimer);
  snackTimer = window.setTimeout(() => (s.hidden = true), 6000);
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
  clearBtn.hidden = true;
  return load(async () => {
    const genres: Genre[] = await Tver.rankingGenres(rankKind);
    if (!genres.length) return empty("ランキングを取得できませんでした");
    if (!genres.some((g) => g.id === rankGenre)) rankGenre = genres[0]!.id;

    const kinds = el("div", "chips");
    for (const [kind, label] of [["series", "番組ランキング"], ["episode", "エピソードランキング"]] as const) {
      const b = el("button", "chip", label);
      b.setAttribute("aria-pressed", String(kind === rankKind));
      b.onclick = () => {
        rankKind = kind;
        showRanking();
      };
      kinds.append(b);
    }

    const chips = el("div", "chips");
    const body = el("div", "ranking-body");
    const buttons = new Map<string, HTMLButtonElement>();
    let seq = 0;
    const render = async (id: string) => {
      const mine = ++seq;
      rankGenre = id;
      buttons.forEach((b, k) => b.setAttribute("aria-pressed", String(k === id)));
      const items = await Tver.ranking(rankKind, id);
      if (mine !== seq) return; // a newer genre was picked meanwhile
      const g = el("div", "grid");
      g.append(...items.map(card));
      body.replaceChildren(items.length ? g : el("p", "muted", "ランキングがありません"));
    };
    for (const g of genres) {
      const b = el("button", "chip", g.title);
      b.onclick = () => load(() => render(g.id));
      buttons.set(g.id, b);
      chips.append(b);
    }
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
  clearBtn.hidden = true;
  return load(async () => {
    const all = await getHome();
    const groups = CATEGORIES.map(([id, label]) => ({
      id,
      label,
      // the home page only carries a top-10 ranking; the full one is added below
      secs: all.filter((s) => s.category === id && !s.title.includes("ランキング")),
    })).filter((g) => g.secs.length);
    if (!groups.length) return empty("カテゴリを取得できませんでした");

    const chips = el("div", "chips");
    const body = el("div", "category-body");
    const buttons: HTMLButtonElement[] = [];
    let shown = -1;
    const select = (i: number) => {
      shown = i;
      buttons.forEach((b, k) => b.setAttribute("aria-pressed", String(k === i)));
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
    groups.forEach((g, i) => {
      const b = el("button", "chip", `${g.label}  ${g.secs.length}`);
      b.onclick = () => select(i);
      buttons.push(b);
      chips.append(b);
    });
    select(0);
    view.replaceChildren(el("h2", "section-title", "カテゴリ"), chips, body);
  });
};

// A series page: one chip per season (本編, メイキング, ...) filtering the episode grids.
function seasons(title: string, secs: Section[], startWithAll = false) {
  const head = el("div", "page-header");
  if (history.length) {
    const back = el("button", "icon-button");
    back.ariaLabel = "戻る";
    back.innerHTML = BACK_ICON;
    back.onclick = goBack;
    head.append(back);
  }
  head.append(el("h2", "section-title", title));

  const chips = el("div", "chips");
  const body = el("div", "season-body");
  const ALL = -1;
  const buttons: HTMLButtonElement[] = [];
  const select = (i: number) => {
    buttons.forEach((b, k) => b.setAttribute("aria-pressed", String(k === i + 1)));
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
  for (const [label, i] of entries) {
    const b = el("button", "chip", label);
    b.onclick = () => select(i);
    buttons.push(b);
    chips.append(b);
  }
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
  return row;
}

function showSettings() {
  setNav("settings");
  history.length = 0;
  input.value = "";
  clearBtn.hidden = true;

  const card = el("section", "settings-card");
  const body = el("div", "settings-body");
  const actions = el("div", "settings-actions");
  const bar = el("div", "linear-progress");
  bar.append(el("div"));
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
      settingsRow("状態", SOURCE_LABEL[info.source as keyof typeof SOURCE_LABEL] ?? info.source),
      settingsRow("バージョン", info.version || "—"),
      settingsRow("場所", info.path || "—"),
    );
    const install = el("button", "filled-button", info.managed ? "最新版に更新" : "ダウンロード");
    install.disabled = busy;
    install.onclick = () => run(() => Tver.ytdlpInstall(), install.textContent!);
    actions.replaceChildren(install);
    if (info.managed) {
      const remove = el("button", "text-button", "アプリ内のコピーを削除");
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

function setNav(page: "home" | "categories" | "ranking" | "search" | "settings") {
  for (const [id, p] of [["#nav-home", "home"], ["#nav-categories", "categories"], ["#nav-ranking", "ranking"], ["#nav-search", "search"], ["#nav-settings", "settings"]] as const) {
    if (p === page) $(id).setAttribute("aria-current", "page");
    else $(id).removeAttribute("aria-current");
  }
}

function goHome() {
  input.value = "";
  clearBtn.hidden = true;
  setNav("home");
  history.length = 0;
  showHome();
}

$("#home").onclick = goHome;
$("#nav-home").onclick = goHome;
$("#nav-categories").onclick = showCategories;
$("#nav-ranking").onclick = showRanking;
$("#nav-search").onclick = () => input.focus();
$("#nav-settings").onclick = showSettings;
clearBtn.onclick = () => {
  input.value = "";
  clearBtn.hidden = true;
  input.focus();
};
input.addEventListener("input", () => (clearBtn.hidden = !input.value));
$("#search").addEventListener("submit", (e) => {
  e.preventDefault();
  const q = input.value.trim();
  if (!q) return goHome();
  setNav("search");
  history.length = 0;
  load(async () => {
    const secs = await Tver.search(q);
    secs.length ? seasons(`「${q}」の検索結果`, secs, true) : empty("見つかりませんでした");
  });
});

// back: Alt+←, the mouse back button, or Esc (the player handles its own Esc)
addEventListener("keydown", (e) => {
  if (playerFull() || e.defaultPrevented) return;
  const typing = e.target instanceof HTMLInputElement;
  if ((e.key === "ArrowLeft" && e.altKey) || (e.key === "Escape" && !typing)) {
    if (goBack()) e.preventDefault();
  }
});
addEventListener("mouseup", (e) => {
  if (e.button === 3 && !playerFull() && goBack()) e.preventDefault();
});

showHome();
