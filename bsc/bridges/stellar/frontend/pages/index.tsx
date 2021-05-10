import React, { useState, useEffect } from 'react'
import { Web3ReactProvider, useWeb3React, UnsupportedChainIdError } from '@web3-react/core'
import {
  NoEthereumProviderError,
  UserRejectedRequestError as UserRejectedRequestErrorInjected
} from '@web3-react/injected-connector'
import { UserRejectedRequestError as UserRejectedRequestErrorWalletConnect } from '@web3-react/walletconnect-connector'
import { UserRejectedRequestError as UserRejectedRequestErrorFrame } from '@web3-react/frame-connector'
import { Web3Provider } from '@ethersproject/providers'
import { Button } from '@material-ui/core'

import { useEagerConnect, useInactiveListener } from '../hooks'
import {
  injected,
} from '../connectors'
import { Spinner } from '../components/Spinner'
import { Balance } from '../components/Balance'
import { Withdraw } from '../components/Withdraw'

import { toast, ToastContainer } from 'react-nextjs-toast'

const CONTRACT_ADDRESS_TESTNET = process.env.CONTRACT_ADDRESS as string
const STELLAR_ENV = process.env.STELLAR_ENV as string
import abi from '../tokenabi.json'
import DepositDialog from '../components/Deposit'
const Contract = require('web3-eth-contract')

enum ConnectorNames {
  Injected = 'Injected',
}

const connectorsByName: { [connectorName in ConnectorNames]: any } = {
  [ConnectorNames.Injected]: injected,
}

function getErrorMessage(error: Error) {
  if (error instanceof NoEthereumProviderError) {
    return 'No Ethereum browser extension detected, install MetaMask on desktop or visit from a dApp browser on mobile.'
  } else if (error instanceof UnsupportedChainIdError) {
    return "You're connected to an unsupported network."
  } else if (
    error instanceof UserRejectedRequestErrorInjected ||
    error instanceof UserRejectedRequestErrorWalletConnect ||
    error instanceof UserRejectedRequestErrorFrame
  ) {
    return 'Please authorize this website to access your Ethereum account.'
  } else {
    console.error(error)
    return 'An unknown error occurred. Check the console for more details.'
  }
}

function getLibrary(provider: any): Web3Provider {
  const library = new Web3Provider(provider)
  library.pollingInterval = 12000
  return library
}

export default function() {
  return (
    <Web3ReactProvider getLibrary={getLibrary}>
      <App />
    </Web3ReactProvider>
  )
}

function App() {
  const context = useWeb3React<Web3Provider>()
  const { connector, account, activate, error, active, library, chainId } = context
  const [loadingWithdrawal, setLoadingWithdrawal] = useState(false)

  const currentConnector = connectorsByName['Injected']

  // handle logic to recognize the connector currently being activated
  const [activatingConnector, setActivatingConnector] = useState<any>()
  useEffect(() => {
    if (activatingConnector && activatingConnector === connector) {
      setActivatingConnector(undefined)
    }
  }, [activatingConnector, connector])

  // handle logic to eagerly connect to the injected ethereum provider, if it exists and has granted access already
  const triedEager = useEagerConnect()

  // handle logic to connect in reaction to certain events on the injected ethereum provider, if it exists
  useInactiveListener(!triedEager || !!activatingConnector)

  const color = active ? 'secondary' : 'primary'
  const connected = currentConnector === connector
  const disabled = !triedEager || !!activatingConnector || connected || !!error
  const activating = currentConnector === activatingConnector

  const [balance, setBalance] = useState()
  useEffect((): any => {
    if (!!account && !!library) {
      let stale = false

      Contract.setProvider(library.provider)
      const contract = new Contract(abi, CONTRACT_ADDRESS_TESTNET)
      
      contract.methods.balanceOf(account).call({ from: account })
        .then(b => {
          if (!stale) {
            setBalance(b)
          }
        })

      return () => {
        stale = true
        setBalance(undefined)
      }
    }
  }, [account, library, chainId]) // ensures refresh if referential identity of library doesn't change across chainIds

  const submitWithdraw = (stellarAddress, amount) => {
    setLoadingWithdrawal(true)
    handleCloseWithdrawDialog()

    Contract.setProvider(library.provider)
    const contract = new Contract(abi, CONTRACT_ADDRESS_TESTNET)

    contract.methods.withdraw(amount*1e7, stellarAddress, STELLAR_ENV).send({ from: account })
      .then(res => {
        toast.notify("Withdrawal success", { type: 'success' })
        contract.methods.balanceOf(account).call({ from: account })
        .then(balance => {
            setLoadingWithdrawal(false)
            setBalance(balance)
          })
        .catch(() => setLoadingWithdrawal(false))
      })
      .catch(() => {
        setLoadingWithdrawal(false)
        toast.notify("Withdrawal failed", { type: 'error' })
      })
  }

  const [openDepositDialog, setOpenDepositDialog] = useState(false)
  const handleCloseDespoitDialog = () => setOpenDepositDialog(false)

  const [openWithdrawDialog, setOpenWithdrawDialog] = useState(false)
  const handleCloseWithdrawDialog = () => setOpenWithdrawDialog(false)

  return (
    <div>
      <ToastContainer />
      <div style={{ display: 'flex', justifyContent: 'flex-end', height: 100, padding: 10 }}>
        {!connected && (<Button
          style={{ height: 50 }}
          variant="contained" 
          color={color} 
          onClick={() => {
            setActivatingConnector(injected)
            activate(injected)
          }}
          disabled={disabled}
        >
          Connect wallet
        </Button>)}
        {activating && <Spinner color={'black'} style={{ height: '25%', marginLeft: '-1rem' }} />}

        {connected && !error && account && (
          <span>{`🟢 ${account.substring(0, 6)}...${account.substring(account.length - 4)}`}</span>
        )}
        {!!error && <h4 style={{ marginTop: '1rem', marginBottom: '0' }}>{getErrorMessage(error)}</h4>}
      </div>
      
      <div style={{ width: '50%', margin: 'auto', textAlign: 'center' }}>
        <img style={{ width: '30%', height: '30%' }} src={'/3fold_logo.png'} />
        <h2>TFT Binance Chain bridge</h2>

        {connected && !error && account && (
          <>
            <div>
              {loadingWithdrawal ? (
                <Spinner color={'black'} style={{ height: '25%', marginLeft: '-1rem' }} />
              ) : (
                <>
                  <Balance balance={balance} />
                </>
              )}
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', marginTop: 20 }}>
              <Button style={{ width: '50%', marginTop: 20, alignSelf: 'center' }} color='primary' variant='outlined' onClick={() => setOpenDepositDialog(true)}>
                Deposit from Stellar
              </Button>
              <DepositDialog address={account} open={openDepositDialog} handleClose={handleCloseDespoitDialog} />
              <Button style={{ width: '50%', marginTop: 20, alignSelf: 'center' }} color='default' variant='outlined' onClick={() => setOpenWithdrawDialog(true)}>
                Withdraw to Stellar
              </Button>
              <Withdraw
                open={openWithdrawDialog}
                handleClose={handleCloseWithdrawDialog}
                balance={balance}
                submitWithdraw={submitWithdraw}
              />
            </div>
          </>
        )}
      </div>

    </div>
  )
}