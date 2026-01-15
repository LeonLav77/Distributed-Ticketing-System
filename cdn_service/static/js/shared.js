export const TICKET_CONTRACT_ADDRESS = '0x254dffcd3277C0b1660F6d42EFbB754edaBAbC2B';
export const MARKETPLACE_CONTRACT_ADDRESS = '0x2F2B2FE9C08d39b1F1C22940a9850e2851F40f99';

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