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
import Logo from '../../../components/icon/logo';
import { Amount, FiatConversion } from '../../../components/rates/rates';
import { translate, TranslateProps } from '../../../decorators/translate';
import { AccountAndBalanceInterface } from './accountssummary';
import { BalanceRow } from './balancerow';
import * as style from './summary.css';

interface ProvidedProps {
    coinCode: string;
    accounts: AccountAndBalanceInterface[];
    total: Amount;
}

type Props = ProvidedProps & TranslateProps;

class BalancesTable extends Component<Props> {
    public render(
        { t, coinCode, accounts, total }: RenderableProps<Props>,
    ) {
        return (
            <div>
                <div className={style.coinheader}>
                    <div className="subHeader" class="row">
                        <div class="flex flex-1 flex-row flex-between flex-items-center flex-center" height={64}>
                            <Logo className={style.coinlogo} coinCode={coinCode} alt={coinCode} active={true} />
                            <h3 className={style.coincode}>{coinCode.toUpperCase()}</h3>
                        </div>
                    </div>
                </div>
                <table className={style.table}>
                    <thead>
                        <tr>
                            <th>{t('accountSummary.name')}</th>
                            <th>{t('accountSummary.balance')}</th>
                            <th>{t('accountSummary.fiatBalance')}</th>
                        </tr>
                    </thead>
                    <tbody>
                        {accounts.map(account => <BalanceRow name={account.name} balance={account.balance.available}/>)}
                    </tbody>
                    <tfoot>
                        <tr>
                            <th>{t('accountSummary.total')}</th>
                            <th>{total.amount}</th>
                            <th><FiatConversion amount={total} /></th>
                        </tr>
                    </tfoot>
                </table>
            </div>
        );
    }
}

const HOC = translate<ProvidedProps>()(BalancesTable);
export { HOC as BalancesTable };
