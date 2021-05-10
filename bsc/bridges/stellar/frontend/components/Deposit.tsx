import React, { useState } from 'react'
import Button from '@material-ui/core/Button'
import Dialog from '@material-ui/core/Dialog'
import DialogActions from '@material-ui/core/DialogActions'
import DialogContent from '@material-ui/core/DialogContent'
import DialogContentText from '@material-ui/core/DialogContentText'
import DialogTitle from '@material-ui/core/DialogTitle'
import QrCode from 'qrcode.react'
import WarningIcon from '@material-ui/icons/Warning'
import { Checkbox, FormControlLabel } from '@material-ui/core'

const BRIDGE_TFT_ADDRESS = process.env.BRIDGE_TFT_ADDRESS

export default function DepositDialog({ open, handleClose, address }) {
  if (!address) return null

  const parsedAddress = Buffer.from(address.replace('0x', ''), 'hex').toString('base64')

  const [checked, setChecked] = useState(false)

  const encodedAddress = encodeURIComponent(parsedAddress)

  return (
    <div>
      <Dialog
        open={open}
        onClose={handleClose}
        aria-labelledby="alert-dialog-title"
        aria-describedby="alert-dialog-description"
        maxWidth={'lg'}
        fullScreen={true}
      >
        <DialogTitle id="alert-dialog-title">{"Swap Stellar TFT for BSC TFT"}</DialogTitle>
        <DialogContent>
          <DialogContentText style={{ margin: 'auto', textAlign: 'center',  width: '50%', display: 'flex', flexDirection: 'column' }}>
            <WarningIcon style={{ alignSelf: 'center', fontSize: 40, color : 'orange' }}/>
            If you want to swap your Stellar TFT to Binance Chain TFT you can transfer any amount to the destination address. 
            Important Note: Please always include the generated memo text for every swap transaction. <b style={{ color: 'red', marginTop: 5 }}>Failure to do so will result in unrecoverable loss of funds!</b>
          </DialogContentText>

          <FormControlLabel
            style={{ margin: 'auto', marginTop: 20, width: '60%', display: 'flex', justifyContent: 'center' }}
            control={
              <Checkbox value={checked} onChange={(e) => setChecked(e.target.checked)} />
            }
            label="I understand that I need to include the generated memo text for every swap transaction, I am responsible for the loss of funds consequence otherwise."
          />

          {checked && (
            <>
              <DialogContentText style={{ margin: 'auto', textAlign: 'center', display: 'flex', flexDirection: 'column', marginTop: 40 }}>
                <span><b>Enter the following information manually:</b></span>
                <span style={{ marginTop: 20 }}>Destination: <b>{BRIDGE_TFT_ADDRESS}</b></span>
                <span>Memo: <b>{parsedAddress}</b></span>
              </DialogContentText>
              <DialogContentText style={{ margin: 'auto', textAlign: 'center', width: '50%', display: 'flex', flexDirection: 'column' }}>
                <h4>
                    Or scan the QR code with Threefold Connect
                </h4>
                
                <QrCode style={{ alignSelf: 'center' }} value={`TFT:${BRIDGE_TFT_ADDRESS}?message=${encodedAddress}&sender=me`} />
              </DialogContentText>
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button style={{ width: 200, height: 50 }} variant='contained' onClick={handleClose} color="primary">
            Close
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}