// i18n: bilingual UI chrome (ru/kk). Authored content (assessment text etc.) is never translated.
// Language persists in localStorage `bip:v1:lang` and is read on init.
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import ruDict from "./ru.json";
import kkDict from "./kk.json";
import { getItem, keys, setItem } from "../lib/storage";

// Feature namespaces: src/i18n/ns/<name>.ru.json and <name>.kk.json are deep-merged over the base dictionaries,
// so each feature owns its own files and never edits ru.json/kk.json.
const nsModules = import.meta.glob("./ns/*.json", { eager: true, import: "default" }) as Record<string, unknown>;

const deepMerge = (target: Record<string, unknown>, src: Record<string, unknown>): Record<string, unknown> => {
  for (const [k, v] of Object.entries(src)) {
    const cur = target[k];
    if (v && typeof v === "object" && !Array.isArray(v) && cur && typeof cur === "object") {
      deepMerge(cur as Record<string, unknown>, v as Record<string, unknown>);
    } else {
      target[k] = v && typeof v === "object" ? structuredClone(v) : v;
    }
  }
  return target;
};

const build = (base: unknown, suffix: ".ru.json" | ".kk.json") => {
  const out = structuredClone(base) as Record<string, unknown>;
  for (const [path, mod] of Object.entries(nsModules)) {
    if (path.endsWith(suffix)) deepMerge(out, mod as Record<string, unknown>);
  }
  return out;
};

const dictionaries = {
  ru: build(ruDict, ".ru.json"),
  kk: build(kkDict, ".kk.json"),
} as const;

export type Lang = keyof typeof dictionaries;
export const LANGS: Lang[] = ["ru", "kk"];
const DEFAULT_LANG: Lang = "ru";

type Params = Record<string, string | number>;

const resolve = (dict: unknown, key: string): string | undefined => {
  let node: unknown = dict;
  for (const p of key.split(".")) {
    if (node == null || typeof node !== "object") return undefined;
    node = (node as Record<string, unknown>)[p];
  }
  return typeof node === "string" ? node : undefined;
};

const interpolate = (str: string, params?: Params): string => {
  if (!params) return str;
  return str.replace(/\{([^}]+)\}/g, (_m, k) => String(params[k.trim()] ?? k));
};

const isLang = (v: unknown): v is Lang => v === "ru" || v === "kk";

const initialLang = (): Lang => {
  const stored = getItem(keys.lang());
  return isLang(stored) ? stored : DEFAULT_LANG;
};

type TranslationContextValue = {
  lang: Lang;
  t: (key: string, params?: Params) => string;
  setLang: (lang: Lang) => void;
};

const TranslationContext = createContext<TranslationContextValue | null>(null);

export const useTranslation = (): TranslationContextValue => {
  const ctx = useContext(TranslationContext);
  if (!ctx) throw new Error("useTranslation must be used within TranslationProvider");
  return ctx;
};

export const TranslationProvider = ({ children }: { children: ReactNode }) => {
  const [lang, setLangState] = useState<Lang>(initialLang);

  useEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);

  const t = useCallback(
    (key: string, params?: Params) => {
      // Missing kk keys fall back to ru, then to the key itself (visible in dev).
      const raw = resolve(dictionaries[lang], key) ?? resolve(dictionaries.ru, key);
      if (raw === undefined) return key;
      return interpolate(raw, params);
    },
    [lang],
  );

  const setLang = useCallback((next: Lang) => {
    if (!isLang(next)) return;
    setLangState(next);
    setItem(keys.lang(), next);
  }, []);

  const value = useMemo(() => ({ lang, t, setLang }), [lang, t, setLang]);

  return React.createElement(TranslationContext.Provider, { value }, children);
};

export { dictionaries };
