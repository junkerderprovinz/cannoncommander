// Layout state of the frame that is not a pref: whether the phone's sidebar sheet is out, how many jobs run, and the
// window width, so layouts that switch at a breakpoint follow a resize.
import { reactive } from 'vue';

export const shell = reactive({ sideOpen: false, jobCount: 0, width: typeof innerWidth === 'number' ? innerWidth : 1600 });

export const watchWidth = () => addEventListener('resize', () => { shell.width = innerWidth; });
