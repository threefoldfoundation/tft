pragma solidity >=0.7.0 <0.9.0;

import "./owned_upgradeable_token_storage.sol";
import "./openzeppelin/contracts/utils/cryptography/ECDSA.sol";

// ----------------------------------------------------------------------------
// Safe maths
// ----------------------------------------------------------------------------
library SafeMath {
    function add(uint a, uint b) internal pure returns (uint c) {
        c = a + b;
        require(c >= a);
    }
    function sub(uint a, uint b) internal pure returns (uint c) {
        require(b <= a);
        c = a - b;
    }
    function mul(uint a, uint b) internal pure returns (uint c) {
        c = a * b;
        require(a == 0 || c / a == b);
    }
    function div(uint a, uint b) internal pure returns (uint c) {
        require(b > 0);
        c = a / b;
    }
}

error InvalidSignature();
error InsufficientSignatures(uint256 numberOfSignatures, uint256 requiredSignatures);

// This represents a validator signature
struct Signature {
	uint8 v;
	bytes32 r;
	bytes32 s;
}

// ----------------------------------------------------------------------------
// ERC20 Token, with the addition of a symbol, name and decimals 
// ----------------------------------------------------------------------------
contract TFT is OwnedUpgradeableTokenStorage {
    using SafeMath for uint;

    event Transfer(address indexed from, address indexed to, uint tokens);
    event Approval(address indexed tokenOwner, address indexed spender, uint tokens);

    // Lets mint some tokens, also index the TFT tx id
    event Mint(address indexed receiver, uint tokens, string indexed txid);
    // Burn tokens in a withdrawal, user chooses how much tokens
    event Withdraw(address indexed receiver, uint tokens, string blockchain_address, string network);

    // name, symbol and decimals getters are optional per the ERC20 spec. Normally auto generated from public variables
    // but that is obviously not going to work for us

    function name() public view returns (string memory) {
        return getName();
    }

    function symbol() public view returns (string memory) {
        return getSymbol();
    }

    function decimals() public view returns (uint8) {
        return getDecimals();
    }

    // ------------------------------------------------------------------------
    // Total supply
    // ------------------------------------------------------------------------
    function totalSupply() public view returns (uint) {
        return getTotalSupply().sub(getBalance(address(0)));
    }


    // ------------------------------------------------------------------------
    // Get the token balance for account `tokenOwner`
    // ------------------------------------------------------------------------
    function balanceOf(address tokenOwner) public view returns (uint balance) {
        return getBalance(tokenOwner);
    }

    // ------------------------------------------------------------------------
    // Transfer the balance from token owner's account to `to` account
    // - Owner's account must have sufficient balance to transfer
    // - 0 value transfers are allowed
    // ------------------------------------------------------------------------
    function transfer(address to, uint tokens) public returns (bool success) {
        setBalance(msg.sender, getBalance(msg.sender).sub(tokens));
        setBalance(to, getBalance(to).add(tokens));
        emit Transfer(msg.sender, to, tokens);
        return true;
    }


    // ------------------------------------------------------------------------
    // Token owner can approve for `spender` to transferFrom(...) `tokens`
    // from the token owner's account
    //
    // https://github.com/ethereum/EIPs/blob/master/EIPS/eip-20-token-standard.md
    // recommends that there are no checks for the approval double-spend attack
    // as this should be implemented in user interfaces 
    // ------------------------------------------------------------------------
    function approve(address spender, uint tokens) public returns (bool success) {
        setAllowed(msg.sender, spender, tokens);
        emit Approval(msg.sender, spender, tokens);
        return true;
    }


    // ------------------------------------------------------------------------
    // Transfer `tokens` from the `from` account to the `to` account
    // 
    // The calling account must already have sufficient tokens approve(...)-d
    // for spending from the `from` account and
    // - From account must have sufficient balance to transfer
    // - Spender must have sufficient allowance to transfer
    // - 0 value transfers are allowed
    // ------------------------------------------------------------------------
    function transferFrom(address from, address to, uint tokens) public returns (bool success) {
        setAllowed(from, msg.sender, getAllowed(from, msg.sender).sub(tokens));
        setBalance(from, getBalance(from).sub(tokens));
        setBalance(to, getBalance(to).add(tokens));
        emit Transfer(from, to, tokens);
        return true;
    }

    // -----------------------------------------------------------------------
    // Withdraw an amount of tokens to another network, these tokens will be burned.
    // -----------------------------------------------------------------------
    function withdraw(uint tokens, string memory blockchain_address, string memory network) public returns (bool success) {
        setBalance(msg.sender, getBalance(msg.sender).sub(tokens));
        setTotalSupply(getTotalSupply().sub(tokens));
        emit Withdraw(msg.sender, tokens, blockchain_address, network);
        return true;
    }


    // ------------------------------------------------------------------------
    // Returns the amount of tokens approved by the owner that can be
    // transferred to the spender's account
    // ------------------------------------------------------------------------
    function allowance(address tokenOwner, address spender) public view returns (uint remaining) {
        return getAllowed(tokenOwner, spender);
    }

    // ------------------------------------------------------------------------
    // Don't accept ETH
    // ------------------------------------------------------------------------
    receive() external payable { }


    // --------------------------------------------------------------------
    // SetSigners sets the set of signer addresses for the mint function
    // @param newSigners the addresses of the new signers 
    // @param signaturesRequired the amount of signatures required to mint
    // --------------------------------------------------------------------
    function SetSigners(address[] calldata newSigners,uint signaturesRequired) external onlyOwner {
        //TODO: check if signaturesRequired is less or equal than the number of signers
       
        address[] storage signersInStorage=getAddresses(keccak256(abi.encode("signers")));
        //clear the set
        for (uint i=0; i<signersInStorage.length - 1; i++)
                signersInStorage.pop();
        //Repopulate it
        for (uint i=0; i<newSigners.length - 1; i++)
                signersInStorage.push(newSigners[i]);
        setAddresses(keccak256(abi.encode("signers")), signersInStorage);
        setUint(keccak256(abi.encode("signaturesRequired")),signaturesRequired);
    }

    // --------------------------------------------------------------------
    // GetSigners returns the set of signer addresses for the mint function
    // and the number of required signatures
    // --------------------------------------------------------------------
    function GetSigners() public view returns (address[] memory) {
        return getAddresses(keccak256(abi.encode("signers")));
    }


    // ------------------------------------------------------------------------------------
    // GetSignaturesRequired return the number of required signatures for the mint function
    // ------------------------------------------------------------------------------------
    function GetSignaturesRequired() public view returns (uint) {
        return getUint(keccak256(abi.encode("signaturesRequired"))) ;
    }


    // Utility function to verify geth style signatures
	function verifySig(
		address _signer,
		bytes32 _theHash,
		Signature calldata _sig
	) private pure returns (bool) {
		bytes32 messageDigest = keccak256(
			abi.encodePacked("\x19Ethereum Signed Message:\n32", _theHash)
		);
		return _signer == ECDSA.recover(messageDigest, _sig.v, _sig.r, _sig.s);
	}

	function checkSignatures(
		// The current signers
		address[] memory _signers,
		// The signatures to verify
		Signature[] calldata _sigs,
		// This is what we are checking they have signed
		uint256 _signaturesRequired,
		// This is what we are checking they have signed
		bytes32 _theHash
	) private pure {
		uint256 cumulativePower = 0;
        
		for (uint256 i = 0; i < _signers.length; i++) {
			// If v is set to 0, this signifies that it was not possible to get a signature from this signer and we skip evaluation
			// (In a valid signature, it is either 27 or 28)
			if (_sigs[i].v != 0) {
				// Check that the current signer has signed off on the hash
				if (!verifySig(_signers[i], _theHash, _sigs[i])) {
					revert InvalidSignature();
				}

				// Sum up cumulative power
				cumulativePower += 1;

				// Break early to avoid wasting gas
				if (cumulativePower >= _signaturesRequired) {
					break;
				}
			}
		}

		// Check that there are enough signatures
		if (cumulativePower < _signaturesRequired) {
			revert InsufficientSignatures(cumulativePower, _signaturesRequired);
		}
		// Success
	}

    // -----------------------------------------------------------------------
    // Mint tokens. Although minting tokens to a withdraw address
    // is just an expensive tft transaction, it is possible, so after minting
    // attemt to withdraw.
    // -----------------------------------------------------------------------
    function mintTokens(address receiver, uint tokens, string memory txid,Signature[] calldata _signatures ) public onlyOwner {
        // check if the txid is already known
        require(!_isMintID(txid), "TFT transacton ID already known");
        bytes32 hashedPayload=keccak256(abi.encode(receiver,tokens,txid));
        
        checkSignatures(GetSigners(),_signatures,GetSignaturesRequired(),hashedPayload);
        _setMintID(txid);
        setBalance(receiver, getBalance(receiver).add(tokens));
        setTotalSupply(getTotalSupply().add(tokens));
        emit Mint(receiver, tokens, txid);
    }

    //------------------------------------------------------
    //Check if minting already occurred for a transaction id
    //------------------------------------------------------
    function isMintID(string memory _txid) public view returns (bool) {
        return _isMintID(_txid);
    }

    function _setMintID(string memory _txid) internal {
        setBool(keccak256(abi.encode("mint","transaction","id",_txid)), true);
    }

    function _isMintID(string memory _txid) internal view returns (bool) {
        return getBool(keccak256(abi.encode("mint","transaction","id", _txid)));
    }
}