/* eslint-disable react-refresh/only-export-components */
import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import { ru, type I18nKey } from "./ru";
import { en } from "./en";

export type { I18nKey };

export const locales = { ru, en } as const;
export type LangCode = keyof typeof locales;

type ContextValue = {
  lang: LangCode;
  setLang: (l: LangCode) => void;
  t: (key: I18nKey, params?: Record<string, string>) => string;
};

const LanguageContext = createContext<ContextValue>({
  lang: "en",
  setLang: () => {},
  t: (key) => key,
});

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<LangCode>(
    () => (localStorage.getItem("lang") as LangCode | null) ?? "en",
  );

  const setLang = useCallback((l: LangCode) => {
    setLangState(l);
    localStorage.setItem("lang", l);
  }, []);

  const t = useCallback((key: I18nKey, params?: Record<string, string>): string => {
    let str =
      (locales[lang] as Record<string, string>)[key] ??
      (locales.ru as Record<string, string>)[key] ??
      key;
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        str = str.replace(`{{${k}}}`, v);
      }
    }
    return str;
  }, [lang]);

  const value = useMemo(() => ({ lang, setLang, t }), [lang, setLang, t]);

  return (
    <LanguageContext.Provider value={value}>
      {children}
    </LanguageContext.Provider>
  );
}

export function useTranslation() {
  return useContext(LanguageContext);
}
