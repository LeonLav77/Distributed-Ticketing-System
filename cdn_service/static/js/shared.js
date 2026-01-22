export const TICKET_CONTRACT_ADDRESS = '0x551D886aD3C536cAFd1550C68CAD139c23a4E7DA';
export const MARKETPLACE_CONTRACT_ADDRESS = '0x0Cd45670669373e538B4023fE53B7fe5A953d07F';

export async function connectMetaMask() {
    if (typeof window.ethereum === 'undefined') {
        throw new Error('MetaMask is not installed! Please install MetaMask to continue.');
    }
    const accounts = await window.ethereum.request({ method: 'eth_requestAccounts' });
    return accounts[0];
}

export function getWeb3() {
    if (typeof window.ethereum === 'undefined') {
        throw new Error('MetaMask is not installed!');
    }
    return new Web3(window.ethereum);
}

export async function fetchMetadata(tokenURI) {
    try {
        const url = tokenURI.replace('ipfs://', 'https://gateway.pinata.cloud/ipfs/');
        const res = await fetch(url);
        return res.ok ? await res.json() : null;
    } catch {
        return null;
    }
}

export async function getConnectedWallet() {
    if (typeof window.ethereum === 'undefined') return null;
    
    try {
        const accounts = await window.ethereum.request({ method: 'eth_accounts' });
        return accounts.length > 0 ? accounts[0] : null;
    } catch {
        return null;
    }
}

export function onAccountsChanged(callback) {
    if (typeof window.ethereum !== 'undefined') {
        window.ethereum.on('accountsChanged', callback);
    }
}