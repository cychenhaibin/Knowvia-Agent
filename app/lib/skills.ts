export const promptFrontMatterPattern = /^---\s*\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/;

export function buildPromptPreview(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return {
      content: '',
      hasFrontMatter: false,
    };
  }

  const frontMatterMatch = trimmed.match(promptFrontMatterPattern);
  const content = frontMatterMatch
    ? trimmed.slice(frontMatterMatch[0].length).trim()
    : trimmed;

  return {
    content: content || trimmed,
    hasFrontMatter: Boolean(frontMatterMatch),
  };
}

export function prefersPromptPreview(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return false;
  }
  if (promptFrontMatterPattern.test(trimmed)) {
    return true;
  }
  return /(^|\n)(#{1,6}\s|[-*]\s|\d+\.\s|>\s|```)/.test(trimmed);
}

export function formatSkillActionError(
  message: string,
  t: (key: any, params?: any) => string,
) {
  const trimmed = message.trim();
  if (!trimmed) {
    return trimmed;
  }

  const lower = trimmed.toLowerCase();
  if (
    lower.includes('skill already exists') ||
    lower.includes('skill definition already exists')
  ) {
    return t('skills.error.alreadyExists');
  }

  if (lower.includes('github archive download failed')) {
    return t('skills.error.githubDownload');
  }

  return trimmed;
}
