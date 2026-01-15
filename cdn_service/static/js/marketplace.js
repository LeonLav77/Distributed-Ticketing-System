import TICKET_ABI from './MintNFTABI.js';
import MARKETPLACE_ABI from './MarketplaceABI.js';
import { 
    TICKET_CONTRACT_ADDRESS, 
    MARKETPLACE_CONTRACT_ADDRESS,
    connectMetaMask,
    getWeb3,
    fetchMetadata,
    getConnectedWallet,
    onAccountsChanged
} from './shared.js';

const web3 = getWeb3();
const marketplaceContract = new web3.eth.Contract(MARKETPLACE_ABI, MARKETPLACE_CONTRACT_ADDRESS);
const ticketContract = new web3.eth.Contract(TICKET_ABI, TICKET_CONTRACT_ADDRESS);

function marketplaceApp() {
    return {
        loading: true,
        error: null,
        walletConnected: false,
        walletAddress: '',
        activeTab: 'browse',
        listings: [],
        myTickets: [],
        showModal: false,
        modalTicket: null,
        modalPrice: '',

        get myListings() {
            return this.listings.filter(listing => 
                listing.seller.toLowerCase() === this.walletAddress.toLowerCase()
            );
        },

        async init() {
            const wallet = await getConnectedWallet();
            if (wallet) {
                this.walletAddress = wallet;
                this.walletConnected = true;
            }

            onAccountsChanged((accounts) => {
                this.walletAddress = accounts[0] || '';
                this.walletConnected = !!accounts[0];
                if (this.walletConnected) { 
                    this.loadData();
                }else{ 
                    this.myTickets = [];
                }
            });

            await this.loadData();
        },

        async connectWallet() {
            try {
                this.walletAddress = await connectMetaMask();
                this.walletConnected = true;
                await this.loadData();
            } catch (error) {
                alert('Failed to connect wallet: ' + error.message);
            }
        },

        async loadData() {
            this.loading = true;
            this.error = null;

            try {
                await Promise.all([
                    this.loadListings(),
                    this.walletConnected ? this.loadMyTickets() : Promise.resolve()
                ]);
            } catch (err) {
                this.error = err.message;
            } finally {
                this.loading = false;
            }
        },

        async loadListings() {
            const events = await marketplaceContract.getPastEvents('TicketListed', {
                fromBlock: 0,
                toBlock: 'latest'
            });

            const tokenIds = [...new Set(events.map(e => e.returnValues.tokenId))];
            const listings = await Promise.all(
                tokenIds.map(async (tokenId) => {
                    const listing = await marketplaceContract.methods.listings(tokenId).call();
                    if (!listing.active) return null;

                    const tokenURI = await ticketContract.methods.tokenURI(tokenId).call();
                    const metadata = await fetchMetadata(tokenURI);

                    return {
                        tokenId,
                        seller: listing.seller,
                        price: listing.price,
                        priceETH: web3.utils.fromWei(listing.price, 'ether'),
                        metadata
                    };
                })
            );

            this.listings = listings.filter(Boolean);
        },

        async loadMyTickets() {
            const events = await ticketContract.getPastEvents('Transfer', {
                filter: { to: this.walletAddress },
                fromBlock: 0,
                toBlock: 'latest'
            });

            const tokenIds = [...new Set(events.map(e => e.returnValues.tokenId))];
            const tickets = await Promise.all(
                tokenIds.map(async (tokenId) => {
                    try {
                        const owner = await ticketContract.methods.ownerOf(tokenId).call();
                        if (owner.toLowerCase() !== this.walletAddress.toLowerCase()) return null;

                        const tokenURI = await ticketContract.methods.tokenURI(tokenId).call();
                        const metadata = await fetchMetadata(tokenURI);
                        const listing = await marketplaceContract.methods.listings(tokenId).call().catch(() => ({ active: false }));

                        return {
                            tokenId,
                            metadata,
                            isListed: listing.active && listing.seller.toLowerCase() === this.walletAddress.toLowerCase(),
                            listingPrice: listing.active ? web3.utils.fromWei(listing.price, 'ether') : '0'
                        };
                    } catch {
                        return null;
                    }
                })
            );

            this.myTickets = tickets.filter(Boolean);
        },

        showListModal(ticket) {
            this.modalTicket = ticket;
            this.modalPrice = ticket.listingPrice || '';
            this.showModal = true;
        },

        showUpdatePriceModal(listing) {
            this.modalTicket = listing;
            this.modalPrice = listing.priceETH;
            this.showModal = true;
        },

        async confirmListTicket() {
            if (!this.modalPrice || parseFloat(this.modalPrice) <= 0) {
                alert('Please enter a valid price');
                return;
            }

            try {
                const priceWei = web3.utils.toWei(this.modalPrice, 'ether');
                const isApproved = await ticketContract.methods.isApprovedForAll(
                    this.walletAddress, 
                    MARKETPLACE_CONTRACT_ADDRESS
                ).call();

                if (!isApproved) {
                    alert('First approval required for marketplace access (one-time)');
                    await ticketContract.methods.setApprovalForAll(MARKETPLACE_CONTRACT_ADDRESS, true)
                        .send({ from: this.walletAddress });
                }

                await marketplaceContract.methods.listTicket(this.modalTicket.tokenId, priceWei)
                    .send({ from: this.walletAddress });

                alert('Ticket listed successfully!');
                this.showModal = false;
                await this.loadData();
            } catch (error) {
                alert('Failed to list ticket: ' + error.message);
            }
        },

        async cancelListing(tokenId) {
            if (!confirm('Cancel this listing?')) return;

            try {
                await marketplaceContract.methods.cancelListing(tokenId)
                    .send({ from: this.walletAddress });
                alert('Listing cancelled!');
                await this.loadData();
            } catch (error) {
                alert('Failed to cancel: ' + error.message);
            }
        },

        async buyTicket(listing) {
            if (!this.walletConnected) {
                alert('Connect wallet first');
                return;
            }

            try {
                await marketplaceContract.methods.buyTicket(listing.tokenId)
                    .send({ from: this.walletAddress, value: listing.price });
                alert('Ticket purchased!');
                await this.loadData();
            } catch (error) {
                alert('Purchase failed: ' + error.message);
            }
        }
    };
}

window.marketplaceApp = marketplaceApp;