// A session names the browser and system it came from, as people recognise their devices by those.
import { c } from '@/features/config/shared';

const BROWSERS: [RegExp, string][] = [[/Edg\//, 'Edge'], [/Firefox\//, 'Firefox'], [/OPR\//, 'Opera'], [/Chrome\//, 'Chrome'], [/Safari\//, 'Safari']];
const SYSTEMS: [RegExp, string][] = [[/Android/, 'Android'], [/iPhone|iPad/, 'iOS'], [/Windows/, 'Windows'], [/Mac OS X/, 'macOS'], [/Linux/, 'Linux']];

export function device(ua: string): { name: string; g: string } {
  const browser = BROWSERS.find(([re]) => re.test(ua))?.[1];
  const system = SYSTEMS.find(([re]) => re.test(ua))?.[1];
  const g = system === 'Android' || system === 'iOS' ? 'IconPhone' : browser ? 'IconBrowser' : 'IconKey';
  if (browser && system) return { name: c('security.browserOn', { browser, system }), g };
  return { name: browser || system || ua || c('security.unknownDevice'), g };
}
