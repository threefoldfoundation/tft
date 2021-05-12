import React, { useState, useEffect } from 'react'
import { FormControl, InputLabel, Input, FormHelperText, Button, Dialog } from '@material-ui/core'
import DialogActions from '@material-ui/core/DialogActions'
import DialogTitle from '@material-ui/core/DialogTitle'
import stellar from 'stellar-sdk'

import styles from './Withdraw.module.scss'

const TFT_ASSET = 'TFT'
const STELLAR_HORIZON_URL = process.env.STELLAR_HORIZON_URL
const TFT_ASSET_ISSUER = process.env.TFT_ASSET_ISSUER
const server = new stellar.Server(STELLAR_HORIZON_URL)

export function Withdraw({ open, handleClose, balance, submitWithdraw }) {
  const [stellarAddress, setStellarAddress] = useState('')
  const [stellarAddressError, setStellarAddressError] = useState('')

  const [amount, setAmount] = useState(0)
  const [amountError, setAmountError] = useState('')

  // Initialize balance
  useEffect(() => {
    if (balance) {
      setAmount(parseInt(balance) / 1e7)
    }
  }, [balance])

  const submit = async () => {
    if (stellarAddress === '') {
      setStellarAddressError('Address not valid')
      return
    }

    try {
      // check if the account provided exists on stellar
      const account = await server.loadAccount(stellarAddress)
      // check if the account provided has the appropriate trustlines
      const includes = account.balances.find(b => b.asset_code === TFT_ASSET && b.asset_issuer === TFT_ASSET_ISSUER)
      if (!includes) {
        setStellarAddressError('Address does not have a valid trustline to TFT')
        return
      }
    } catch (error) {
      setStellarAddressError('Address not found')
      return
    }

    if (amount <= 0 || amount > balance / 1e7) {
      setAmountError('Amount not valid')
      return
    }

    setStellarAddressError('')
    setAmountError('')

    submitWithdraw(stellarAddress, amount)
  }

  const handleStellarAddressChange = (e) => {
    setStellarAddressError('')
    setStellarAddress(e.target.value)
  }

  const handleAmountChange = (e) => {
    setAmountError('')
    try {
      const a = parseInt(e.target.value)
      if (isNaN(a)) {
        setAmount(0)
      } else {
        setAmount(a)
      }
    } catch (err) {
      setAmountError(err)
    }
  }

  return (
    <div>
      <Dialog
        open={open}
        onClose={handleClose}
        aria-labelledby="alert-dialog-title"
        aria-describedby="alert-dialog-description"
        fullScreen={true}
      >
        <DialogTitle id="alert-dialog-title">{"Swap BSC TFT for Stellar TFT"}</DialogTitle>
        <div className={styles.container}>
          <span>Fill in this form to withdraw tokens back to Stellar</span>
          <FormControl>
          <InputLabel htmlFor="StellarAddress">Stellar Address</InputLabel>
          <Input 
              value={stellarAddress}
              onChange={handleStellarAddressChange}
              id="StellarAddress"
              aria-describedby="my-helper-text"
          />
          <FormHelperText id="my-helper-text">Enter a valid Stellar Address</FormHelperText>
          {stellarAddressError && (
              <div className={styles.errorField}>{stellarAddressError}</div>
          )}
          </FormControl>

          <FormControl>
          <InputLabel htmlFor="StellarAddress">Amount</InputLabel>
          <Input 
              value={amount}
              onChange={handleAmountChange}
              id="amount"
              aria-describedby="my-helper-text"
              type='float'
          />
          <FormHelperText id="my-helper-text">Enter an amount, balance: {balance / 1e7}</FormHelperText>
          {amountError && (
              <div className={styles.errorField}>{amountError}</div>
          )}
          </FormControl>

          <Button 
            color='primary'
            variant="contained"
            style={{ marginTop: 25 }}
            type='submit'
            onClick={() => submit()}
            >
            Withdraw
          </Button>
        </div>
        <DialogActions>
          <Button style={{ width: 200, height: 50 }} variant='contained' onClick={handleClose} color="primary">
            Close
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}