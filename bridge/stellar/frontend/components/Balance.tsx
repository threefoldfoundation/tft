export function Balance ({ balance }) {
  if (!balance) return (
    <span>Connect your wallet to continue</span>
  )

  return (
    <>
      <span>Your TFT Balance</span>
      <span role="img" aria-label="gold">
        💰
      </span>
      <span>{balance === null ? 'Error' : balance ? `${formatBalance(balance)}` : ''}</span>
    </>
  )
}

const formatBalance = (balance) => {
  return parseInt(balance) / 1e7
}