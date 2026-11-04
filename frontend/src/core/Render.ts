// Draws whatever an extension point handed back: a vnode, a string or nothing.
import type { FunctionalComponent, VNodeChild } from 'vue';

export const Render: FunctionalComponent<{ node: VNodeChild }> = (props) => props.node as ReturnType<FunctionalComponent>;
Render.props = ['node'];
