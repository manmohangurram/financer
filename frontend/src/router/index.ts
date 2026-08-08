import { createRouter, createWebHistory } from 'vue-router';
import LoginView from '@/views/LoginView.vue';
import SignupView from '@/views/SignupView.vue';
import AccountsView from '@/views/AccountsView.vue';
import AccountsManageView from '@/views/AccountsManageView.vue';
import RulesView from '@/views/RulesView.vue';
import CategoriesView from '@/views/CategoriesView.vue';
import InvestmentsView from '@/views/InvestmentsView.vue';
import InvestmentDetailView from '@/views/InvestmentDetailView.vue';
import ProfileView from '@/views/ProfileView.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView },
    { path: '/signup', component: SignupView },
    {
      path: '/',
      component: () => import('@/components/AppLayout.vue'),
      children: [
        { path: '', redirect: '/accounts' },
        { path: 'accounts', name: 'accounts', component: AccountsView },
        { path: 'accounts/manage', name: 'accounts-manage', component: AccountsManageView },
        { path: 'accounts/rules', name: 'rules', component: RulesView },
        { path: 'accounts/categories', name: 'categories', component: CategoriesView },
        { path: 'investments', name: 'investments', component: InvestmentsView },
        { path: 'investments/:id', name: 'investment-detail', component: InvestmentDetailView },
        { path: 'settings', name: 'settings', component: ProfileView }
      ]
    },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
});

export default router;
