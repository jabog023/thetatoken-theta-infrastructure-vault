package handler

// func TestSend(t *testing.T) {
// 	assert := assert.New(t)
// 	et := execution.NewExecTest()
// 	aliceSa := ttypes.MakeAccWithInitBalance("alice_sa", ttypes.NewCoins(100000, 200000))
// 	aliceRa := ttypes.MakeAccWithInitBalance("alice_ra", ttypes.NewCoins(300000, 400000))
// 	bobRa := ttypes.MakeAccWithInitBalance("bob_ra", ttypes.NewCoins(0, 0))
// 	et.SetAcc(aliceRa, aliceSa, bobRa)

// 	thetaAmount, _ := ttypes.ParseCoinAmount("123")
// 	gammaAmount, _ := ttypes.ParseCoinAmount("456")
// 	sendArgs := &SendArgs{
// 		To: bobRa.PubKey.Address().Hex(),
// 		Amount: ttypes.Coins{
// 			ThetaWei: thetaAmount,
// 			GammaWei: gammaAmount,
// 		},
// 		Sequence: 1,
// 	}
// 	alice := db.Record{
// 		UserID:       "alice",
// 		RaAddress:    aliceRa.PubKey.Address(),
// 		RaPrivateKey: aliceRa.PrivKey,
// 		RaPubKey:     aliceRa.PubKey,
// 		SaAddress:    aliceSa.PubKey.Address(),
// 		SaPrivateKey: aliceSa.PrivKey,
// 		SaPubKey:     aliceSa.PubKey,
// 	}
// 	signedTx, err := prepareSendTx(sendArgs, alice, "test_chain_id")
// 	assert.Nil(err)
// 	assert.Equal(int64(0), signedTx.Fee.ThetaWei.Int64())
// 	assert.Equal(ttypes.MinimumTransactionFeeGammaWei, signedTx.Fee.GammaWei.Uint64())

// 	_, res := et.Executor().ScreenTx(signedTx)
// 	assert.True(res.IsOK())

// 	_, res = et.Executor().ExecuteTx(signedTx)
// 	assert.True(res.IsOK())

// 	endAliceRABalance := et.State().Delivered().GetAccount(aliceRa.PubKey.Address()).Balance
// 	assert.Equal(int64(300000-123), endAliceRABalance.ThetaWei.Int64())
// 	assert.Equal(int64(400000-456-1), endAliceRABalance.GammaWei.Int64())

// 	endAliceSABalance := et.State().Delivered().GetAccount(aliceSa.PubKey.Address()).Balance
// 	assert.Equal(int64(100000), endAliceSABalance.ThetaWei.Int64())
// 	assert.Equal(int64(200000), endAliceSABalance.GammaWei.Int64())

// 	endBobRABalance := et.State().Delivered().GetAccount(bobRa.PubKey.Address()).Balance
// 	assert.Equal(int64(123), endBobRABalance.ThetaWei.Int64())
// 	assert.Equal(int64(456), endBobRABalance.GammaWei.Int64())
// }

// func TestReserveFund(t *testing.T) {
// 	assert := assert.New(t)
// 	et := execution.NewExecTest()
// 	aliceSa := ttypes.MakeAccWithInitBalance("alice_sa", ttypes.NewCoins(100000, 200000))
// 	aliceRa := ttypes.MakeAccWithInitBalance("alice_ra", ttypes.NewCoins(300000, 400000))
// 	bobRa := ttypes.MakeAccWithInitBalance("bob_ra", ttypes.NewCoins(0, 0))
// 	et.SetAcc(aliceRa, aliceSa, bobRa)

// 	reserveFundArgs := &ReserveFundArgs{
// 		Collateral:  "9000",
// 		Fund:        "4500",
// 		ResourceIds: []string{"Die_another_day"},
// 		Sequence:    1,
// 		Duration:    500,
// 	}
// 	alice := db.Record{
// 		UserID:       "alice",
// 		RaAddress:    aliceRa.PubKey.Address(),
// 		RaPrivateKey: aliceRa.PrivKey,
// 		RaPubKey:     aliceRa.PubKey,
// 		SaAddress:    aliceSa.PubKey.Address(),
// 		SaPrivateKey: aliceSa.PrivKey,
// 		SaPubKey:     aliceSa.PubKey,
// 	}
// 	signedTx, err := prepareReserveFundTx(reserveFundArgs, alice, "test_chain_id")
// 	assert.Nil(err)
// 	assert.Equal(int64(0), signedTx.Fee.ThetaWei.Int64())
// 	assert.Equal(ttypes.MinimumTransactionFeeGammaWei, signedTx.Fee.GammaWei.Uint64())

// 	_, res := et.Executor().ScreenTx(signedTx)
// 	assert.True(res.IsOK())

// 	_, res = et.Executor().ExecuteTx(signedTx)
// 	assert.True(res.IsOK())

// 	endAliceRABalance := et.State().Delivered().GetAccount(aliceRa.PubKey.Address()).Balance
// 	assert.Equal(int64(300000), endAliceRABalance.ThetaWei.Int64())
// 	assert.Equal(int64(400000), endAliceRABalance.GammaWei.Int64())

// 	endAliceSABalance := et.State().Delivered().GetAccount(aliceSa.PubKey.Address()).Balance
// 	assert.Equal(int64(100000), endAliceSABalance.ThetaWei.Int64())
// 	assert.Equal(int64(200000-9000-4500-1), endAliceSABalance.GammaWei.Int64())
// }

// func TestServicePayment(t *testing.T) {
// 	assert := assert.New(t)
// 	et := execution.NewExecTest()
// 	aliceSa := ttypes.MakeAccWithInitBalance("alice_sa", ttypes.NewCoins(100000, 200000))
// 	aliceRa := ttypes.MakeAccWithInitBalance("alice_ra", ttypes.NewCoins(300000, 400000))
// 	bobRa := ttypes.MakeAccWithInitBalance("bob_ra", ttypes.NewCoins(0, 0))
// 	bobSa := ttypes.MakeAccWithInitBalance("bob_sa", ttypes.NewCoins(0, 0))
// 	et.SetAcc(aliceRa, aliceSa, bobRa)

// 	// 1. Reserve fund.
// 	reserveFundArgs := &ReserveFundArgs{
// 		Collateral:  9000,
// 		Fund:        4500,
// 		ResourceIds: []string{"Die_another_day"},
// 		Sequence:    1,
// 		Duration:    500,
// 	}
// 	alice := db.Record{
// 		UserID:       "alice",
// 		RaAddress:    aliceRa.PubKey.Address(),
// 		RaPrivateKey: aliceRa.PrivKey,
// 		RaPubKey:     aliceRa.PubKey,
// 		SaAddress:    aliceSa.PubKey.Address(),
// 		SaPrivateKey: aliceSa.PrivKey,
// 		SaPubKey:     aliceSa.PubKey,
// 	}
// 	bob := db.Record{
// 		UserID:       "bob",
// 		RaAddress:    bobRa.PubKey.Address(),
// 		RaPrivateKey: bobRa.PrivKey,
// 		RaPubKey:     bobRa.PubKey,
// 		SaAddress:    bobSa.PubKey.Address(),
// 		SaPrivateKey: bobSa.PrivKey,
// 		SaPubKey:     bobSa.PubKey,
// 	}

// 	signedTx, err := prepareReserveFundTx(reserveFundArgs, alice, "test_chain_id")
// 	assert.Nil(err)
// 	_, res := et.Executor().ExecuteTx(signedTx)
// 	assert.True(res.IsOK())

// 	endAliceRA := et.State().Delivered().GetAccount(aliceRa.PubKey.Address())
// 	endAliceRABalance := endAliceRA.Balance
// 	assert.Equal(int64(300000), endAliceRABalance.ThetaWei.Int64())
// 	assert.Equal(int64(400000), endAliceRABalance.GammaWei.Int64())

// 	endAliceSA := et.State().Delivered().GetAccount(aliceSa.PubKey.Address())
// 	endAliceSABalance := endAliceSA.Balance
// 	assert.Equal(int64(100000), endAliceSABalance.ThetaWei.Int64())
// 	assert.Equal(int64(200000-9000-4500-1), endAliceSABalance.GammaWei.Int64())

// 	// 2. Create service payment.
// 	createServicePaymentArgs := &CreateServicePaymentArgs{
// 		To:              bobRa.PubKey.Address().Hex(),
// 		Amount:          123,
// 		ResourceId:      "Die_another_day",
// 		PaymentSequence: 1,
// 		ReserveSequence: 1,
// 	}
// 	paymentTx, err := prepareCreateServicePaymentTx(createServicePaymentArgs, alice, "test_chain_id")
// 	assert.Nil(err)

// 	// 3. Submit service payment.
// 	submitServiePaymentArgs := &SubmitServicePaymentArgs{
// 		Payment:  paymentTx,
// 		Sequence: 1,
// 	}
// 	submitPaymentTx, err := prepareSubmitServicePaymentTx(submitServiePaymentArgs, bob, "test_chain_id")
// 	assert.Nil(err)

// 	_, res = et.Executor().ExecuteTx(submitPaymentTx)
// 	assert.True(res.IsOK())

// 	endBobRABalance := et.State().Delivered().GetAccount(bobRa.PubKey.Address()).Balance
// 	assert.Equal(int64(0), endBobRABalance.ThetaWei.Int64())
// 	expected := big.NewInt(123)
// 	expected.Sub(expected, new(big.Int).SetUint64(ttypes.MinimumTransactionFeeGammaWei))
// 	assert.Equal(0, expected.Cmp(endBobRABalance.GammaWei))
// }
