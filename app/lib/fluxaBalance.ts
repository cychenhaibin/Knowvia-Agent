import type {FluxABalance} from '@/types/api';

function isNonNegativeFinite(value: number): boolean {
  return Number.isFinite(value) && value >= 0;
}

function isPositiveFinite(value: number): boolean {
  return Number.isFinite(value) && value > 0;
}

export function formatFluxABalance(balance: FluxABalance, locale = 'en-US'): string | null {
  if (!isNonNegativeFinite(balance.quota)) {
    return null;
  }

  try {
    switch (balance.quotaDisplayType) {
      case 'USD':
        if (!isPositiveFinite(balance.quotaPerUnit)) {
          return null;
        }
        return new Intl.NumberFormat(locale, {
          style: 'currency',
          currency: 'USD',
          currencyDisplay: 'symbol',
          maximumFractionDigits: 2,
        }).format(balance.quota / balance.quotaPerUnit);
      case 'CNY':
        if (!isPositiveFinite(balance.quotaPerUnit) || !isPositiveFinite(balance.usdExchangeRate)) {
          return null;
        }
        return new Intl.NumberFormat(locale, {
          style: 'currency',
          currency: 'CNY',
          currencyDisplay: 'symbol',
          maximumFractionDigits: 2,
        }).format((balance.quota / balance.quotaPerUnit) * balance.usdExchangeRate);
      case 'CUSTOM':
        if (
          !isPositiveFinite(balance.quotaPerUnit) ||
          !isPositiveFinite(balance.customCurrencyExchangeRate) ||
          !balance.customCurrencySymbol.trim()
        ) {
          return null;
        }
        return `${balance.customCurrencySymbol}${new Intl.NumberFormat(locale, {
          maximumFractionDigits: 2,
        }).format((balance.quota / balance.quotaPerUnit) * balance.customCurrencyExchangeRate)}`;
      case 'TOKENS':
        return new Intl.NumberFormat(locale, {maximumFractionDigits: 2}).format(balance.quota);
      default:
        return null;
    }
  } catch {
    return null;
  }
}
