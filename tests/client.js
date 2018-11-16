const request = require('request');
const BN = require('bn.js');

const Logger = console;

const DEFAULT_CONFIG = {
    unitPayment: 230e12 * 5,             // Payment per chunk = 230 GammaWei * 5 secs.
    reserveBatchSize: 36,             // = reserved_amount / unit_payment = 3 min / 5 secs.
    submitPaymentInterval: 60 * 1000, // Interval of payment submission  = 1 min.
    reserveFundTTL: 300 * 1000,       // Theta backend default expiration time = 300 secs. 
};

function isNullBN(a) {
    return BN.isBN(a) && (a.eq(new BN(null)) || a.eq(new BN(undefined)));
}

// Wallet base class.
class BaseWalletService {
    constructor(config) {
        this.config = Object.assign({}, DEFAULT_CONFIG, config);

        // Latest on-chain account state. Call this.getAccount() to update.
        this.account = null;

        // Sending side handles of off-chain payment channels. A map from resourceIds to reserves.
        // Reserve is in following form:
        //   {
        //        sequence: [number]    // Reserve sequence.
        //        channels: [Map[string->Channel]] // Address to channel.
        //        balance:  [number]    // Remaining balance.
        //        expiresAt: [number]   // Timestamp when reserve should stop to be used.
        //   }
        // Channel is in following form:
        //   {
        //       sequence: [number]     // Payment sequence.
        //       accumulation: [number] // Amount that has been sent so far.
        //   }
        this.sendChannels = new Map();

        // Receiving side handles of off-chain payment channels. A multi-levels map from
        // (sourceAddress, reserveSequence, reserveSequence::paymentSequence) to payment.
        // Payment is in following form:
        //   {
        //       resourceId: [number],
        //       sourceAddress: [string],
        //       targetAddress: [string],
        //       paymentHex: [string],
        //       reserveSequence: [number],
        //       paymentSequence: [number]
        //   }
        // In addition, a map (sourceAddress, reserveSequence) =>
        // (lastActivePaymentSeqence, lastActiveReserveSequence) is maintained for invoice
        //  generation.
        this.receiveChannels = new Map();
    }

    setContext(ctx) {
        this.context = ctx;
    }

    async start() {
        this._submit_timer = setInterval(this._submitReceivedPayments.bind(this),
            this.config.submitPaymentInterval);

        await this._getAccount();
    }

    stop() {
        clearInterval(this._submit_timer);
    }

    // -------------- High level payment interface -------------

    async getAccount() {
        return this._getAccount();
    }

    //
    // createInvoice is invoked on the payment receiver to create an invoice, which then should be
    // sent to payment sender. The invoice specifies the preferred payment channel to be reused, but
    // the sender might create new payment channel due to insufficient fund in the preferred
    // channel.
    //
    createInvoice(sourceAddress, resourceId) {
        if (!this.account) {
            return null;
        }
        let lastActivePaymentSequence = null;
        let lastActiveReserveSequence = null;
        let bySourceAddress = this.receiveChannels.get(sourceAddress);
        if (bySourceAddress) {
            let byResourceId = bySourceAddress.get(resourceId);
            if (byResourceId) {
                lastActiveReserveSequence = byResourceId.lastActiveReserveSequence;
                lastActivePaymentSequence = byResourceId.lastActivePaymentSequence;
            }
        }
        return {
            resourceId: resourceId,
            address: this.account.recv_account.address,
            paymentSequence: lastActivePaymentSequence,
            reserveSequence: lastActiveReserveSequence
        };
    }

    //
    // createPayment creates a payment object to be sent to receiver.
    // invoice is created by createInvoice(). If reserveSequence and paymentSequence matches with
    // existing channel and remaining fund is sufficient, the existing channel is reused. Otherwise
    // a new channel is created.
    //
    async createPayment(invoice) {
        if (!this.account) {
            return null;
        }

        let { resourceId, address, reserveSequence, paymentSequence } = invoice;
        let reserveSequenceBN = new BN(reserveSequence);
        let paymentSequenceBN = new BN(paymentSequence);
        let paymentAmountBN = new BN(this.config.unitPayment);
        let reserveAmountBN = paymentAmountBN.mul(new BN(this.config.reserveBatchSize));
        let collateralBN = reserveAmountBN.add(new BN(1e12));

        let reservedFund = this.sendChannels.get(resourceId);
        if (!reservedFund || reservedFund.balance.lt(paymentAmountBN) || reservedFund.expiresAt <= (new Date().getTime())) {
            // Create new reserve.
            let reserve = await this._reserveFund(resourceId, reserveAmountBN, collateralBN);
            reservedFund = {
                sequence: new BN(reserve.reserve_sequence),
                channels: new Map(), // Mapping addresses to channels.
                balance: reserveAmountBN,

                // When a reserved fund is about to expire, it should stop to be used to pay.
                // Adding 5 secs buffer so that peer's payment submission has enough time to be included in blockchain.
                expiresAt: (new Date().getTime()) + this.config.reserveFundTTL - this.config.submitPaymentInterval - 5 * 1000
            };
            this.sendChannels.set(resourceId, reservedFund);
        }

        let channel = reservedFund.channels.get(address);
        // Create new channel if channel has not been created or existing channel doesn't
        // match with invoice.
        if (!channel || !reserveSequenceBN.eq(reservedFund.sequence) ||
            isNullBN(paymentSequenceBN) || !channel.sequence.eq(paymentSequenceBN)) {
            // Timestamp based nonce
            paymentSequenceBN = new BN(Date.now());
            channel = {
                accumulation: new BN(0),
                sequence: paymentSequenceBN
            };
            reservedFund.channels.set(address, channel);
        }
        let paymentResp = await this._createPayment(
            address, channel.accumulation.add(paymentAmountBN), reservedFund.sequence, channel.sequence, resourceId);

        channel.accumulation = channel.accumulation.add(paymentAmountBN);
        reservedFund.balance = reservedFund.balance.sub(paymentAmountBN);

        return {
            resourceId: resourceId,
            sourceAddress: this.account.recv_account.address,
            targetAddress: address,
            amount: paymentAmountBN,
            paymentHex: paymentResp.payment,
            reserveSequence: reservedFund.sequence,
            paymentSequence: channel.sequence
        };
    }

    //
    // receivePayment saves a payment object. Saved payments are peridically submitted by
    // _submitReceivedPayments(). The payment object is the output of createPayment().
    //
    async receivePayment(payment) {
        if (!this.account) {
            return null;
        }

        // TODO: Parse and verify payment. resourceId and sourceAddress should be verified against
        // paymentHex.
        let { resourceId, sourceAddress, paymentSequence, reserveSequence } = payment;

        let channelsByAddress = this.receiveChannels.get(sourceAddress);
        if (!channelsByAddress) {
            channelsByAddress = new Map();
            this.receiveChannels.set(sourceAddress, channelsByAddress);
        }

        let channelsByResourceId = channelsByAddress.get(resourceId);
        if (!channelsByResourceId) {
            channelsByResourceId = {
                lastActivePaymentSequence: null,
                lastActiveReserveSequence: null,
                channels: new Map()
            };
            channelsByAddress.set(resourceId, channelsByResourceId);
        }

        channelsByResourceId.lastActivePaymentSequence = paymentSequence;
        channelsByResourceId.lastActiveReserveSequence = reserveSequence;
        channelsByResourceId.channels.set(`${reserveSequence}::${paymentSequence}`, payment);
    }

    // getGammaBalance returns Gamma balance in the cached account.
    getGammaBalance() {
        let getGamma = (acc) => {
            if (acc.coins != null) {
                return acc.coins.gammawei
            }
            return -1;
        };
        if (!this.account) {
            return {
                sa: -1,
                ra: -1
            };
        }
        return {
            sa: getGamma(this.account.send_account),
            ra: getGamma(this.account.recv_account)
        };
    }

    //
    // ------------------ Private methods --------------------
    //

    // ----------------- Abstract methods --------------------

    async _makeRPC(name, args) {
        throw 'Not implemented';
    }

    // --------------------- RPC calls -----------------------

    // _RPC wraps over _makeRPC to count RPC results.
    async _RPC(name, args) {
        try {
            let res = await this._makeRPC(name, args);
            if (this.context) {
                this.context.playerStats.walletRPCSucceeded(`${name}:ok`);
                if (name == 'theta.ReserveFund') {
                    this.context.playerStats.walletReserveSucceeded();
                } else if (name == 'theta.SubmitServicePayment') {
                    this.context.playerStats.walletSubmitPaymentSucceeded();
                }
            }
            return res;
        } catch (e) {
            if (this.context) {
                let result = null;
                if (e.message) {
                    result = /;([^;}]*?)}/.exec(e.message)[1];
                    if (result) {
                        result = result.substring(0, 20);
                    }
                }
                result = result || 'unknown_error';
                this.context.playerStats.walletRPCFailed(`${name}:${result}`);
                if (name == 'theta.ReserveFund') {
                    this.context.playerStats.walletReserveFailed();
                } else if (name == 'theta.SubmitServicePayment') {
                    this.context.playerStats.walletSubmitPaymentFailed();
                }
            }
            throw e;
        }
    }

    async _getAccount() {
        let result = await this._RPC('theta.GetAccount');
        this.account = result;

        if (this.context) {
            this.context.trigger(Theta.Events.ACCOUNT_UPDATED, { account: result });
        }

        return this.account;
    }

    async _reserveFund(resourceId, amount, collateral) {
        await this._getAccount();
        let sequence = new BN(this.account.send_account.sequence || 0),
            amountBN = new BN(amount),
            collateralBN = new BN(collateral);

        return this._RPC('theta.ReserveFund', {
            collateral: collateralBN.toString(10),
            fund: amountBN.toString(10),
            resource_ids: [resourceId],
            sequence: sequence.add(new BN(1)).toString(10)
        });
    }

    async _createPayment(address, amount, reserveSeq, paymentSeq, resourceId) {
        return this._RPC('theta.CreateServicePayment', {
            to: address,
            amount: new BN(amount).toString(10),
            reserve_sequence: new BN(reserveSeq).toString(10),
            payment_sequence: new BN(paymentSeq).toString(10),
            resource_id: resourceId
        });
    }

    async _submitPayment(payment) {
        await this._getAccount();
        let sequence = new BN(this.account.recv_account.sequence || 0);

        return this._RPC('theta.SubmitServicePayment', {
            to: this.account.recv_account.address,
            payment: payment,
            sequence: sequence.add(new BN(1)).toString(10)
        });
    }


    //
    // ----------------- utilities ----------------------
    //

    //
    // _submitPayment submit and clear saved payments. This is periodically executed.
    //
    async _submitReceivedPayments() {
        for (let [sourceAddress, bySourceAddress] of this.receiveChannels) {
            for (let [resourceId, byResourceId] of bySourceAddress) {
                for (let [key, payment] of byResourceId.channels) {
                    try {
                        // Logger.log(`Submitting payment: resourceId: ${resourceId}, sourceAddress: ` +
                        //     `${sourceAddress}, payment: ${JSON.stringify(payment)}`);

                        let res = await this._submitPayment(payment.paymentHex);

                        // Logger.log('Payment result: ', res);
                    } catch (e) {
                        // Logger.warn(`Error submitting payment: error: ${JSON.stringify(e)}, ` +
                        //     `resourceId: ${resourceId}, sourceAddress: ${sourceAddress},` +
                        //     ` payment: ${JSON.stringify(payment)}`);
                    }
                    // Mark payment channel as closed.
                    byResourceId.channels.delete(key);
                    let expectedKey = `${byResourceId.lastActiveReserveSequence}::` +
                        `${byResourceId.lastActivePaymentSequence}`;
                    if (expectedKey == key) {
                        byResourceId.lastActivePaymentSequence = null;
                        byResourceId.lastActiveReserveSequence = null;
                    }
                }
            }
        }
    }
}

// SliverWallet implementation.
class SliverWallet extends BaseWalletService {
    constructor(config) {
        super(config);
        this.auth = config;
        this.endpoint = 'http://localhost:20000/rpc';

    }
    getAuth() {
        return this.auth;
    }
    async _makeRPC(name, args) {
        let body = {
            "jsonrpc": "2.0",
            "method": name,
            "params": [args],
            "id": "1"
        };
        return new Promise((resolve, reject) => {
            let auth = this.getAuth();
            let headers = {
                'X-Auth-User': auth.xuser,
                'X-Auth-Token': auth.xtoken,
                'Content-Type': 'application/json'
            };
            request.post(
                this.endpoint,
                {
                    json: body,
                    headers: headers,
                },
                function (error, response, body) {
                    if (error || response.statusCode != 200) {
                        reject(error)
                        return
                    }
                    if (body.error) {
                        reject(body.error)
                        return
                    }
                    resolve(body.result);
                }
            );
        });
    }
}

module.exports = SliverWallet