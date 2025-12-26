type AuthorInfo = {
  authorEmail?: string | null;
  authorFirstName?: string | null;
  authorLastName?: string | null;
};

export const formatDate = (value: string) =>
  new Date(value).toLocaleString("ru-RU", {
    timeZone: "Europe/Moscow",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

export const formatAuthor = (author: AuthorInfo) => {
  const first = (author.authorFirstName ?? "").trim();
  const last = (author.authorLastName ?? "").trim();
  const fullName = [first, last].filter(Boolean).join(" ").trim();
  if (fullName) {
    return fullName;
  }
  return author.authorEmail || "Не указан";
};
