// Whether job windows show the bar or open up into the graph, one choice for all of them.
import { prefs, setPref } from '@/core/prefs';

export const toggleDetails = () => setPref('jobDetails', prefs.jobDetails !== true);
