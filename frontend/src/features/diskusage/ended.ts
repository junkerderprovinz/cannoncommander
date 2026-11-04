// Counts the jobs that ended, so an open disk usage window knows when to measure again.
import { ref } from 'vue';
import { useJobs } from '@/stores/jobs';

export const ended = ref(0);
export const countEnded = () => useJobs().onEnded(() => { ended.value++; });
