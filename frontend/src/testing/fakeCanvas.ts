// jsdom draws nothing on a canvas and measures no words, so this stands in for both and keeps a log of what would be
// drawn: the words, the filled grounds, the splinters cut from a picture and the sparks.
export interface Drawn { texts: string[]; grounds: number; splinters: number; strokes: number }

export function fakeCanvas() {
  const drawn: Drawn = { texts: [], grounds: 0, splinters: 0, strokes: 0 };
  const pattern = {} as CanvasPattern;
  const g = {
    fillStyle: '' as unknown,
    scale() {}, translate() {}, setTransform() {}, clearRect() {}, beginPath() {}, closePath() {}, moveTo() {}, lineTo() {}, roundRect() {}, drawImage() {},
    fillRect() { drawn.grounds++; },
    fill() {
      if (g.fillStyle === pattern) drawn.splinters++;
      else drawn.grounds++;
    },
    stroke() { drawn.strokes++; },
    fillText(text: string) { drawn.texts.push(text); },
    measureText: () => ({ fontBoundingBoxAscent: 11, fontBoundingBoxDescent: 3 }),
    createPattern: () => pattern,
  };
  const context = HTMLCanvasElement.prototype.getContext, rect = Range.prototype.getBoundingClientRect;
  HTMLCanvasElement.prototype.getContext = (() => g) as unknown as typeof context;
  Range.prototype.getBoundingClientRect = function () {
    return (this.startContainer.parentElement ?? this.startContainer as Element).getBoundingClientRect();
  };
  const reset = () => { drawn.texts = []; drawn.grounds = drawn.splinters = drawn.strokes = 0; };
  const restore = () => {
    HTMLCanvasElement.prototype.getContext = context;
    if (rect) Range.prototype.getBoundingClientRect = rect;
    else delete (Range.prototype as Partial<Range>).getBoundingClientRect;
  };
  return { drawn, reset, restore };
}
