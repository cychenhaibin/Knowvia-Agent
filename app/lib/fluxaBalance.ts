import type {FluxABalance} from '@/types/api';

function isValidBalance(balance: FluxABalance): boolean {
  return (
    Number.isFinite(balance.quota) &&
    balance.quota >= 0 &&
    Number.isFinite(balance.quotaPerUnit) &&
    balance.quotaPerUnit > 0 &&
    Number.isFinite(balance.usdExchangeRate) &&
    balance.usdExchangeRate > 0 &&
    Number.isFinite(balance.customCurrencyExchangeRate) &&
    balance.customCurrencyExchangeRate > 0
  );
}

export function formatFluxABalance(balance: FluxABalance, locale = 'en-US'): string | null {
  if (!isValidBalance(balance)) {
    return null;
  }

  const units = balance.quota / balance.quotaPerUnit;

  try {
    switch (balance.quotaDisplayType) {
      case 'USD':
        return new Intl.NumberFormat(locale, {
          style: 'currency',
          currency: 'USD',
          currencyDisplay: 'symbol',
          maximumFractionDigits: 2,
        }).format(units);
      case 'CNY':
        return new Intl.NumberFormat(locale, {
          style: 'currency',
          currency: 'CNY',
          currencyDisplay: 'symbol',
          maximumFractionDigits: 2,
        }).format(units * balance.usdExchangeRate);
      case 'CUSTOM':
        if (!balance.customCurrencySymbol.trim()) {
          return null;
        }
        return `${balance.customCurrencySymbol}${new Intl.NumberFormat(locale, {
          maximumFractionDigits: 2,
        }).format(units * balance.customCurrencyExchangeRate)}`;
      case 'TOKENS':
        return new Intl.NumberFormat(locale, {maximumFractionDigits: 2}).format(balance.quota);
      default:
        return null;
    }
  } catch {
    return null;
  }
}
