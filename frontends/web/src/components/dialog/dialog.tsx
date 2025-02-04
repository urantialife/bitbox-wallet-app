/**
 * Copyright 2018 Shift Devices AG
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Component, h, RenderableProps } from 'preact';
import * as style from './dialog.css';

interface Props {
    title?: string;
    small?: boolean;
    large?: boolean;
    disableEscape?: boolean;
    onClose?: (e: Event) => void;
}

interface State {
    active: boolean;
    currentTab: number;
}

class Dialog extends Component<Props, State> {
    private modalContent!: HTMLElement;
    private focusableChildren!: NodeListOf<HTMLElement>;

    constructor(props: Props) {
        super(props);
        this.state = {
            active: false,
            currentTab: 0,
        };
    }

    public componentDidMount() {
        setTimeout(this.activate, 10);
    }

    public componentWillUnmount() {
        this.deactivate();
    }

    private setModalContent = (element: HTMLElement) => {
        this.modalContent = element;
    }

    private handleFocus = (e: FocusEvent) => {
        const input = e.target as HTMLElement;
        const index = input.getAttribute('index');
        this.setState({ currentTab: Number(index) });
    }

    private focusWithin = () => {
        this.focusableChildren = this.modalContent.querySelectorAll('a, button, input, textarea');
        const focusables = Array.from(this.focusableChildren);
        for (const c of focusables) {
            c.classList.add('tabbable');
            c.setAttribute('index', focusables.indexOf(c).toString());
            c.addEventListener('focus', this.handleFocus);
        }
        document.addEventListener('keydown', this.handleKeyDown);
    }

    private focusFirst = () => {
        const focusables = this.focusableChildren;
        if (focusables.length) {
            focusables[0].focus();
        }
    }

    private updateIndex = (isNext: boolean) => {
        const target = this.getNextIndex(isNext);
        this.setState({ currentTab: target }, () => {
            this.focusableChildren[target].focus();
        });
    }

    private getNextIndex(isNext: boolean) {
        const { currentTab } = this.state;
        const focusables = Array.from(this.focusableChildren);
        const arr = isNext ? focusables : focusables.reverse();
        const current = isNext ? currentTab : (arr.length - 1) - currentTab;
        let next = isNext ? currentTab + 1 : arr.length - currentTab;
        next = arr.findIndex((item, i) => (i >= next && !item.hasAttribute('disabled')));
        next = next < 0 ? arr.findIndex((item, i) => (i <= current && !item.hasAttribute('disabled'))) : next;
        return isNext ? next : (arr.length - 1) - next;
    }

    private handleKeyDown = (e: KeyboardEvent) => {
        const { disableEscape, onClose } = this.props;
        const isEsc = e.keyCode === 27;
        const isTab = e.keyCode === 9;
        if (!disableEscape && isEsc) {
            if (onClose) {
                onClose(e);
            }
        } else if (isTab) {
            e.preventDefault();
        }
        if (isTab && e.shiftKey) {
            this.updateIndex(false);
        } else if (isTab) {
            this.updateIndex(true);
        }
    }

    private deactivate = () => {
        this.setState({
            active: false,
            currentTab: 0,
        });
        document.removeEventListener('keydown', this.handleKeyDown);
    }

    private activate = () => {
        this.setState({ active: true }, () => {
            this.focusWithin();
            this.focusFirst();
        });
    }

    public render(
        { title, small, large, children }: RenderableProps<Props>,
        { active }: State,
    ) {
        const activeClass = active ? style.active : '';
        return (
            <div class={[style.overlay, activeClass].join(' ')}>
                <div class={[style.modal, activeClass, small ? style.small : '', large ? style.large : ''].join(' ')}>
                    {
                        title && (
                            <h3 class={style.modalHeader}>{title}</h3>
                        )
                    }
                    <div class={[style.modalContent, title ? '' : 'first'].join(' ')} ref={this.setModalContent}>
                        {children}
                    </div>
                </div>
            </div>
        );
    }
}

export { Dialog };
